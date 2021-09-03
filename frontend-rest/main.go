package main

import (
	"cloud.google.com/go/compute/metadata"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	pb "github.com/kazshinohara/pb/whereami"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	port     = os.Getenv("PORT")
	version  = os.Getenv("VERSION")
	kind     = os.Getenv("KIND")
	backend = os.Getenv("BE")
	backend_port = os.Getenv("BE_PORT")
)

type commonResponse struct {
	Kind     string `json:"kind"`    // backend, backend-b, backend-c
	Version  string `json:"version"` // v1, v2, v3
	Region   string `json:"region"`
	Cluster  string `json:"cluster"`
	Hostname string `json:"hostname"`
}

func resolveRegion() string {
	if !metadata.OnGCE() {
		log.Println("This app is not running on GCE")
	} else {
		zone, err := metadata.Zone()
		if err != nil {
			log.Printf("could not get zone info: %v", err)
			return "unknown"
		}
		region := zone[:strings.LastIndex(zone, "-")]
		return region
	}
	return "unknown"
}

func resolveCluster() string {
	if !metadata.OnGCE() {
		log.Println("This app is not running on GCE")
	} else {
		cluster, err := metadata.Get("/instance/attributes/cluster-name")
		if err != nil {
			log.Printf("could not get cluster name: %v", err)
			return "unknown"
		}
		return cluster
	}
	return "unknown"
}

func resolveHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("could not get hostname: %v", err)
		return "unknown"
	}
	return hostname
}

func fetchBackend(target string, path string) *commonResponse {
	conn, err := grpc.Dial(target + ":" + backend_port, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewWhereamiClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 60)
	defer cancel()
	r, err := c.GetServerInfo(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatalf("could not get hostinfo: %v", err)
	}

	time.Sleep(time.Second * 1)

	return &commonResponse{
		Version: r.Version,
		Kind: r.Kind,
		Region: r.Region,
		Cluster: r.Cluster,
		Hostname: r.Hostname,
	}
}

func fetchRootResponse(w http.ResponseWriter, r *http.Request) {
	responseBody, err := json.Marshal(&commonResponse{
		Version:  version,
		Kind:     kind,
		Region:   resolveRegion(),
		Cluster:  resolveCluster(),
		Hostname: resolveHostname(),
	})
	if err != nil {
		log.Printf("could not json.Marshal: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.Write(responseBody)
}

func fetchBackendResponse(w http.ResponseWriter, r *http.Request) {
	backendRes := fetchBackend(backend, "")

	responseBody, err := json.Marshal(backendRes)
	if err != nil {
		log.Printf("could not json.Marshal: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBody)
}

func main() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", fetchRootResponse).Methods("GET")
	router.HandleFunc("/backend", fetchBackendResponse).Methods("GET")
	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatal("ListenAndServer: ", err)
	}
}
