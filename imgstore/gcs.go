package imgstore

import (
	"errors"
	"io"
	"net/http"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"google.golang.org/appengine"
	"google.golang.org/appengine/file"
	"google.golang.org/appengine/log"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

var bucket string

func Create(filename string, r *http.Request) (*storage.Object, error) {
	c := appengine.NewContext(r)
	bucket, err := file.DefaultBucketName(c)
	if err != nil {
		log.Errorf(c, "Failed to get default bucket: %v", err)
		return nil, err
	}

	ctx, err := auth(r)
	if err != nil {
		log.Errorf(c, "Failed to get context: %v", err)
		return nil, err
	}

	log.Infof(c, "Recieved post with content length %v", r.ContentLength)

	w := storage.NewWriter(ctx, bucket, filename)

	file, header, err := r.FormFile("image")
	if err != nil {
		log.Errorf(c, "Failed to read form file: %v", err)
		return nil, err
	}

	log.Infof(c, "File Header:\nFilename = %v\nHeader Data = %v", header.Filename, header.Header)

	sample := make([]byte, 512)
	read, err := file.Read(sample)
	if err != nil {
		log.Warningf(c, "Failed to sniff content type from file sample: %v", err)
	} else {
		ct := http.DetectContentType(sample)
		log.Infof(c, "Sniffed content type: %v", ct)
		valid := validateContentType(ct)
		if !valid {
			log.Warningf(c, "Invalid Content-Type '%v'. Aborting upload.", ct)
			return nil, errors.New("Invalid file type.")
		}
		w.ContentType = ct
	}

	log.Infof(c, "Writing %v sample bytes to file.", read)
	sampleWritten, err := w.Write(sample)
	if err != nil {
		log.Errorf(c, "Error during write of file sample bytes. Wrote %v. %v", sampleWritten, err)
		w.Close()
		return nil, err
	}

	log.Infof(c, "Copying remainder of file...")
	written, err := io.Copy(w, file)

	log.Infof(c, "Done. Wrote %v bytes.", written)

	err = w.Close()
	if err != nil {
		log.Errorf(c, "Failed to close writer: %v", err)
		return nil, err
	}

	obj := w.Object()

	return obj, nil
}

func Read(filename string, w http.ResponseWriter, r *http.Request) error {
	c := appengine.NewContext(r)

	rc, err := Reader(filename, r)
	if err != nil {
		return err
	}

	defer rc.Close()

	read, err := io.Copy(w, rc)
	if err != nil {
		log.Errorf(c, "Failed to create reader for file: %v", err)
		return err
	}

	log.Infof(c, "Read %v bytes.", read)

	return nil
}

func Reader(filename string, r *http.Request) (io.ReadCloser, error) {
	c := appengine.NewContext(r)
	bucket, err := file.DefaultBucketName(c)
	if err != nil {
		log.Errorf(c, "Failed to get default bucket: %v", err)
		return nil, err
	}

	ctx, err := auth(r)
	if err != nil {
		log.Errorf(c, "Failed to get context: %v", err)
		return nil, err
	}

	log.Infof(c, "Retrieving file %v from bucket %v.", filename, bucket)

	rc, err := storage.NewReader(ctx, bucket, filename)
	if err != nil {
		log.Errorf(c, "Failed to open file: %v", err)
		return nil, err
	}

	return rc, nil
}

func Writer(filename string, r *http.Request) (*storage.Writer, error) {
	c := appengine.NewContext(r)
	bucket, err := file.DefaultBucketName(c)
	if err != nil {
		log.Errorf(c, "Failed to get default bucket: %v", err)
		return nil, err
	}

	ctx, err := auth(r)
	if err != nil {
		log.Errorf(c, "Failed to get context: %v", err)
		return nil, err
	}

	w := storage.NewWriter(ctx, bucket, filename)

	return w, nil
}
func MediaLink(filename string, r *http.Request) (string, error) {
	obj, err := FileStats(filename, r)
	if err != nil {
		c := appengine.NewContext(r)
		log.Errorf(c, "Failed to get file stats: %v", err)
		return "", err
	}

	return obj.MediaLink, nil
}

func Filetype(filename string, r *http.Request) (string, error) {
	obj, err := FileStats(filename, r)
	if err != nil {
		c := appengine.NewContext(r)
		log.Errorf(c, "Failed to get file stats: %v", err)
		return "", err
	}

	return obj.ContentType, nil
}

func FileStats(filename string, r *http.Request) (*storage.Object, error) {
	c := appengine.NewContext(r)
	bucket, err := file.DefaultBucketName(c)
	if err != nil {
		log.Errorf(c, "Failed to get default bucket: %v", err)
		return nil, err
	}

	ctx, err := auth(r)
	if err != nil {
		log.Errorf(c, "Failed to get context: %v", err)
		return nil, err
	}

	log.Infof(c, "Getting stats for file %v from bucket %v.", filename, bucket)

	obj, err := storage.StatObject(ctx, bucket, filename)
	if err != nil {
		log.Errorf(c, "Failed to stat file: %v", err)
		return nil, err
	}

	return obj, nil
}

// Delete removes an object by name from the bucket being used. If the object does not
// exist and there is nothing to delete, Delete returns with no error.
func Delete(filename string, r *http.Request) error {
	c := appengine.NewContext(r)
	bucket, err := file.DefaultBucketName(c)
	if err != nil {
		log.Errorf(c, "Failed to get default bucket: %v", err)
		return err
	}

	ctx, err := auth(r)
	if err != nil {
		log.Errorf(c, "Failed to get context: %v", err)
		return err
	}

	log.Infof(c, "Attempting to delete file %v from bucket %v.", filename, bucket)

	// StatObject is used here to check existence before calling DeleteObject.
	// If the object does not exist, DeleteObject returns an error that is NOT
	// ErrObjectNotExist, so it seemed more reliable to check with StatObject first...
	_, err = storage.StatObject(ctx, bucket, filename)
	if err == storage.ErrObjectNotExist {
		log.Warningf(c, "Object does not exist, nothing to delete.")
		return nil
	}

	err = storage.DeleteObject(ctx, bucket, filename)
	if err != nil {
		log.Errorf(c, "Failed to delete file.")

		log.Infof(c, "Attempting to remove public access to file...")
		aclErr := storage.DeleteACLRule(ctx, bucket, filename, storage.AllUsers)
		if aclErr != nil {
			log.Errorf(c, "Failed to remove public file access!")
		} else {
			log.Infof(c, "File access removed.")
		}

		return err
	}

	return nil
}

func ObjectLink(obj *storage.Object) string {
	return "https://storage.googleapis.com/" + obj.Bucket + "/" + obj.Name
}

func validateContentType(filetype string) bool {
	if filetype == "" {
		return false
	}

	return filetype == "image/png" || filetype == "image/jpg" ||
		filetype == "image/jpeg" || filetype == "image/gif"
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
