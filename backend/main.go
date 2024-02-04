package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"

	"cloud.google.com/go/firestore"
	"github.com/emicklei/go-restful/v3"

	firebase "firebase.google.com/go"
	storage "firebase.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	firebaseConfigFile = "karepoints-firebase-adminsdk-hksnk-5d57163f0c.json"
	firebaseDBURL      = "https://karepoints.firebaseio.com"
)

var (
	ctx context.Context
	app *firebase.App
	fs  *firestore.Client
	cs  *storage.Client
)

type ListPerson struct {
	Name   string `json:"name"`
	Points int64  `json:"points"`
	Photo  []byte `json:"photo"`
}

func main() {
	var err error
	config := &firebase.Config{
		StorageBucket: "karepoints.appspot.com",
	}
	ctx = context.Background()
	opt := option.WithCredentialsFile(firebaseConfigFile)
	app, err = firebase.NewApp(ctx, config, opt)
	if err != nil {
		log.Fatalf("Firebase initialization error: %v\n", err)
	}

	// Initialize Firestore
	fs, err = app.Firestore(ctx)
	if err != nil {
		log.Fatalf("Firestore initialization error: %v\n", err)
	}

	cs, err = app.Storage(ctx)
	if err != nil {
		log.Fatalf("Firestore initialization error: %v\n", err)
	}

	ws := new(restful.WebService)

	ws.Route(ws.GET("/list").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON).
		To(handleList))

	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedDomains: []string{},
		AllowedMethods: []string{"POST"},
		CookiesAllowed: true,
		Container:      restful.DefaultContainer}

	restful.DefaultContainer.Filter(cors.Filter)
	restful.Add(ws)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleList(req *restful.Request, resp *restful.Response) {
	people := make([]ListPerson, 0)
	bucket, err := cs.DefaultBucket()

	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	iter := fs.Collection("people").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Failed to iterate: %v", err)
		}

		data := doc.Data()
		reader, err := bucket.Object(data["photo"].(string)).NewReader(ctx)
		if err != nil {
			resp.WriteError(http.StatusInternalServerError, err)
			return
		}

		photo, err := io.ReadAll(reader)
		if err != nil {
			resp.WriteError(http.StatusInternalServerError, err)
			return
		}

		people = append(people, ListPerson{
			Name:   doc.Ref.ID,
			Points: data["points"].(int64),
			Photo:  photo,
		})
	}

	sort.Slice(people, func(i, j int) bool {
		return people[i].Points < people[j].Points
	})

	marshalled, err := json.Marshal(people)
	if err != nil {
		log.Printf("Could not marshall list")
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = resp.Write(marshalled)
	if err != nil {
		log.Printf("Writing list to response error %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	return
}
