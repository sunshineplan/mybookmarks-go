package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sunshineplan/utils/archive"
	"github.com/sunshineplan/utils/mail"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func addUser(username string) {
	log.Println("Start!")
	if err := initDB(); err != nil {
		log.Fatalln("Failed to initialize database:", err)
	}

	username = strings.TrimSpace(strings.ToLower(username))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := collAccount.InsertOne(ctx, bson.D{
		{Key: "username", Value: username},
		{Key: "password", Value: "123456"},
		{Key: "uid", Value: username},
	})
	if err != nil {
		log.Fatal(err)
	}
	objectID, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		log.Fatal("Failed to get last insert id.")
	}
	if _, err := collBookmark.InsertOne(ctx, bson.D{
		{Key: "bookmark", Value: "Google"},
		{Key: "url", Value: "https://www.google.com"},
		{Key: "user", Value: objectID.Hex()},
		{Key: "seq", Value: 1},
	}); err != nil {
		log.Fatal(err)
	}
	log.Println("Done!")
}

func deleteUser(username string) {
	log.Println("Start!")
	if err := initDB(); err != nil {
		log.Fatalln("Failed to initialize database:", err)
	}

	username = strings.TrimSpace(strings.ToLower(username))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := collAccount.DeleteOne(ctx, bson.M{"username": username})
	if err != nil {
		log.Fatalln("Failed to delete user:", err)
	} else if res.DeletedCount == 0 {
		log.Fatalf("User %s does not exist.", username)
	}
	log.Println("Done!")
}

func reorderBookmark(userID interface{}, orig, dest primitive.ObjectID) error {
	var origBookmark, destBookmark bookmark

	c := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		c <- collBookmark.FindOne(ctx, bson.M{"_id": orig}).Decode(&origBookmark)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := collBookmark.FindOne(ctx, bson.M{"_id": dest}).Decode(&destBookmark); err != nil {
		return err
	}
	if err := <-c; err != nil {
		return err
	}

	var filter, update bson.M
	if origBookmark.Seq > destBookmark.Seq {
		filter = bson.M{"user": userID, "seq": bson.M{"$gte": destBookmark.Seq, "$lt": origBookmark.Seq}}
		update = bson.M{"$inc": bson.M{"seq": 1}}
	} else {
		filter = bson.M{"user": userID, "seq": bson.M{"$gt": origBookmark.Seq, "$lte": destBookmark.Seq}}
		update = bson.M{"$inc": bson.M{"seq": -1}}
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := collBookmark.UpdateMany(ctx, filter, update); err != nil {
		log.Println("Failed to reorder bookmark:", err)
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := collBookmark.UpdateOne(
		ctx, bson.M{"seq": destBookmark.Seq}, bson.M{"_id": orig}); err != nil {
		log.Println("Failed to reorder bookmark:", err)
		return err
	}

	return nil
}

func backup() {
	log.Println("Start!")
	var config struct {
		SMTPServer     string
		SMTPServerPort int
		From, Password string
		To             []string
	}
	if err := meta.Get("mybookmarks_backup", &config); err != nil {
		log.Fatalln("Failed to get mybookmarks_backup metadata:", err)
	}
	dialer := &mail.Dialer{
		Host:     config.SMTPServer,
		Port:     config.SMTPServerPort,
		Account:  config.From,
		Password: config.Password,
	}

	tmpfile, err := ioutil.TempFile("", "tmp")
	if err != nil {
		log.Fatalln("Failed to create temporary file:", err)
	}
	tmpfile.Close()
	if err := dbConfig.Backup(tmpfile.Name()); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	b, err := ioutil.ReadFile(tmpfile.Name())
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	if err := archive.Pack(&buf, archive.ZIP, archive.File{Name: "database", Body: b}); err != nil {
		log.Fatal(err)
	}
	if err := dialer.Send(
		&mail.Message{
			To:          config.To,
			Subject:     fmt.Sprintf("My Bookmarks Backup-%s", time.Now().Format("20060102")),
			Attachments: []*mail.Attachment{{Bytes: buf.Bytes(), Filename: "backup.zip"}},
		},
	); err != nil {
		log.Fatalln("Failed to send mail:", err)
	}
	log.Println("Done!")
}

func restore(file string) {
	log.Println("Start!")
	if file == "" {
		file = joinPath(dir(self), "scripts/schema.sql")
	} else {
		if _, err := os.Stat(file); err != nil {
			log.Fatalln("File not found:", err)
		}
	}
	dropAll := joinPath(dir(self), "scripts/drop_all.sql")
	if err := dbConfig.Restore(dropAll); err != nil {
		log.Fatal(err)
	}
	if err := dbConfig.Restore(file); err != nil {
		log.Fatal(err)
	}
	log.Println("Done!")
}
