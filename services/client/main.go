package main

import (
	//"context"
	//"encoding/json"

	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/HORNET-Storage/go-hornet-storage-lib/lib/connmgr"
	"github.com/HORNET-Storage/go-hornet-storage-lib/lib/signing"
	merkle_dag "github.com/HORNET-Storage/scionic-merkletree/dag"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
)

// These are example keys generated for the purpose of this test client
// Please do not use them for anything other than this
const npub string = "npub1qwf2wtfyprlnta3nk5r4kryq9x0m0pyhuaj0ne2dsdnk053ghak5sg0jkrr"
const nsec string = "nsec1083wvgge0h4f446g43j7gp04ukd3dvjqh9m773jveq4ms84tj74qph3tcu"

func main() {
	ctx := context.Background()

	RunCommandWatcher(ctx)
}

func RunCommandWatcher(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan

		Cleanup(ctx)
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		scanner.Scan()

		command := strings.TrimSpace(scanner.Text())
		segments := strings.Split(command, " ")

		switch segments[0] {
		case "help":
			log.Println("Available Commands:")
			log.Println("upload")
			log.Println("download")
			log.Println("shutdown")
		case "upload":
			UploadDag(ctx, segments[1])
		case "download":
			DownloadDag(ctx, segments[1])
		case "shutdown":
			log.Println("Shutting down")
			Cleanup(ctx)
			return
		default:
			log.Printf("Unknown command: %s\n", command)
		}
	}
}

func Cleanup(ctx context.Context) {

}

func UploadDag(ctx context.Context, path string) {
	// Create a new dag from a directory
	dag, err := merkle_dag.CreateDag(path, true)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	// Verify the entire dag
	err = dag.Verify()
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	log.Println("Dag verified correctly")

	decodedKey, err := signing.DecodeKey(npub)
	if err != nil {
		log.Fatal(err)
	}

	publicKey, err := crypto.UnmarshalSecp256k1PublicKey(decodedKey)
	if err != nil {
		log.Fatal(err)
	}

	peerId, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	ctx, client, err := connmgr.Connect(ctx, fmt.Sprintf("/ip4/127.0.0.1/udp/9000/quic-v1/p2p/%s", peerId.String()), npub, libp2p.Transport(libp2pquic.NewTransport))
	if err != nil {
		log.Fatal(err)
	}

	jsonData, _ := dag.ToJSON()
	os.WriteFile("before_upload.json", jsonData, 0644)

	//IterateDag(dag, func(leaf *merkle_dag.DagLeaf) {
	//	log.Printf("Processing leaf: %s\n", leaf.Hash)
	//})

	privateKey, _, err := signing.DeserializePrivateKey(nsec)
	if err != nil {
		log.Fatal(err)
	}

	signature, err := signing.SignCID(cid.MustParse(dag.Root), privateKey)
	if err != nil {
		log.Fatal(err)
	}

	serializedSignature := hex.EncodeToString(signature.Serialize())

	pubKey := npub

	// Upload the dag to the hornet storage node
	_, err = client.UploadDag(ctx, dag, &pubKey, &serializedSignature)
	if err != nil {
		log.Fatal(err)
	}

	// Disconnect client as we no longer need it
	client.Disconnect()
}

func DownloadDag(ctx context.Context, root string) {
	// Connect to a hornet storage node
	decodedKey, err := signing.DecodeKey(npub)
	if err != nil {
		log.Fatal(err)
	}

	publicKey, err := crypto.UnmarshalSecp256k1PublicKey(decodedKey)
	if err != nil {
		log.Fatal(err)
	}

	peerId, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	ctx, client, err := connmgr.Connect(ctx, fmt.Sprintf("/ip4/127.0.0.1/udp/9000/quic-v1/p2p/%s", peerId.String()), npub, libp2p.Transport(libp2pquic.NewTransport))
	if err != nil {
		log.Fatal(err)
	}

	// Upload the dag to the hornet storage node
	_, dag, err := client.DownloadDag(ctx, root, nil, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the entire dag
	err = dag.Verify()
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	log.Println("Dag verified correctly")

	jsonData, _ := json.Marshal(dag)
	os.WriteFile("after_download.json", jsonData, 0644)

	err = dag.CreateDirectory("D:/organizations/akashic_record/relevant/golang/output")
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	// Disconnect client as we no longer need it
	client.Disconnect()
}
