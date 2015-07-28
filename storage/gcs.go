package storage

import (
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"google.golang.org/appengine"
	"google.golang.org/appengine/file"
	"google.golang.org/appengine/log"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"

	"net/http"
)

var bucket string

func Create(filename string, r *http.Request) error {
	c := appengine.NewContext(r)
	bucket, err := file.DefaultBucketName(c)
	if err != nil {
		log.Errorf(c, "Failed to get default bucket: %v", err)
		return err
	}

	ctx, err := auth(r)
	if err != nil {
		log.Errorf(c, "Failed to get context... %v", err)
		return err
	}

	wc := storage.NewWriter(ctx, bucket, filename)
	wc.ContentType = "text/plain"
	wc.Metadata = map[string]string{
		"x-sample-meta": "some-value",
	}

	_, err = wc.Write([]byte("abcde\n"))
	if err != nil {
		log.Errorf(c, "Failed to write data to file %v in bucket %v: %v", filename, bucket, err)
		return err
	}

	err = wc.Close()
	if err != nil {
		log.Errorf(c, "Failed to close writer: %v", err)
		return err
	}

	return nil
}

func Read() ([]byte, error) {
	return nil, nil
}

func Remove() (bool, error) {
	return false, nil
}

func auth(r *http.Request) (context.Context, error) {
	c := appengine.NewContext(r)
	client, err := google.DefaultClient(c, storage.ScopeFullControl)
	if err != nil {
		return nil, err
	}

	ctx := cloud.NewContext(appengine.AppID(c), client)

	return ctx, nil
}
