package imgproc

import (
	"appengine"
	"github.com/reedperry/gogram/imgstore"
	"net/http"
)

func init() {
	http.Handle("/", http.HandlerFunc(ProcessImage))
}

func ProcessImage(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if isAppEngineModuleRequest(r) {
		return
	}

	if r.Header.Get("X-AppEngine-QueueName") == "" {
		c.Errorf("Request missing required header for a Task Queue request. Processing aborted.")
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		c.Errorf("Form Value 'filename' is missing or empty.")
		http.Error(w, "Failed to process image.", http.StatusBadRequest)
		return
	}

	filetype, err := imgstore.Filetype(filename, r)
	if err != nil {
		c.Errorf("Cannot process image %v: %v", filename, err)
		http.Error(w, "Failed to process image.", http.StatusInternalServerError)
		return
	}

	c.Infof("Processing image %v of type %v.", filename, filetype)

	// Read file
	reader, err := imgstore.Reader(filename, r)
	if err != nil {
		c.Errorf("Failed to open file %v: %v", filename, err)
		http.Error(w, "Failed to process image.", http.StatusInternalServerError)
		return
	}

	defer reader.Close()

	newName := thumbnailFilename(filename)
	writer, err := imgstore.Writer(newName, r)
	if err != nil {
		c.Errorf("Failed to open new file %v for writing: %v", newName, err)
		http.Error(w, "Failed to process image.", http.StatusInternalServerError)
		return
	}

	defer writer.Close()

	writer.ContentType = filetype

	c.Infof("Creating thumbnail %v of type %v from file %v.", newName, filetype, filename)

	if err = makeThumbnail(filetype, reader, writer); err != nil {
		c.Errorf("Failed to create thumbnail from image %v: %v", filename, err)
		http.Error(w, "Failed to process image.", http.StatusInternalServerError)
		return
	}

	c.Infof("Created thumbnail %v.", newName)
}

func thumbnailFilename(filename string) string {
	return filename + "_thumb"
}

func isAppEngineModuleRequest(r *http.Request) bool {
	return r.Method == "GET" &&
		(r.URL.Path == "/_ah/start" || r.URL.Path == "/_ah/stop")
}
