package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/upload", upload)
	http.ListenAndServe(":8080", nil)
}

func upload(w http.ResponseWriter, r *http.Request) {
	// Parse incoming form, allow 100MB in memory
	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Error parsing form: %v", err)))
	}

	formData := r.MultipartForm
	images := formData.File["image"]

	format := r.FormValue("format")
	switch format {
	case "png", "jpg", "jpeg", "gif":

	case "":
		w.Write([]byte(fmt.Sprintf("No file format given :(")))

	default:
		w.Write([]byte(fmt.Sprintf("File format %v not supported :(", format)))
	}

	// Handle 0, 1, 2+ files uploaded
	switch len(images) {
	case 0:
		w.Write([]byte(fmt.Sprintf("No files given :(")))
		return

	// If only 1 file uploaded, don't need to zip/tar it, just send back the converted image
	case 1:
		image, err := images[0].Open()
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Couldn't process file: %v", images[0].Filename)))
			return
		}

		convImage, err := convert(format, image)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Error converting %v to %v :(", images[0].Filename, format)))
			return
		}

		newFilename := replaceExt(images[0].Filename, format)

		err = sendFile(convImage, newFilename, w)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Couldn't send back new file %v :(", newFilename)))
			return
		}
		return

	// Multiple images to handle, need to zip/tar them
	default:
		imageBytes := []tarBuffer{}
		for _, header := range images {
			image, err := header.Open()
			if err != nil {
				// Just skip images that are broken
				continue
			}
			// Might leave this here, just in case
			// defer image.Close()

			convertedImg, err := convert(format, image)
			if err != nil {
				// Just skip images that are broken
				image.Close()
				continue
			}
			imageBytes = append(imageBytes, tarBuffer{Contents: &convertedImg, Filename: replaceExt(header.Filename, format)})
			image.Close()
		}

		// Skipped all the files due to errors
		if len(imageBytes) < 1 {
			w.Write([]byte(fmt.Sprintf("No suitable images uploaded :(")))
			return
		}

		tar, err := tarFiles(&imageBytes)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Unable to tar requested pictures: %v :(", err)))
			return
		}

		err = sendFile(tar, "images.tar", w)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Unable to send back requested pictures: %v :(", err)))
		}
		return
	}

}

func convert(format string, f io.Reader) ([]byte, error) {
	img, _, err := image.Decode(f)
	if err != nil {
		return []byte{}, err
	}

	buf := bytes.NewBuffer(nil)

	switch format {
	case "png":
		err = png.Encode(buf, img)
		if err != nil {
			return []byte{}, err
		}
	case "jpeg", "jpg":
		err = jpeg.Encode(buf, img, &jpeg.Options{Quality: 100})
		if err != nil {
			return []byte{}, err
		}
	case "gif":
		err = gif.Encode(buf, img, nil)
		if err != nil {
			return []byte{}, err
		}
	}

	return buf.Bytes(), nil
}

func replaceExt(filename, extension string) string {
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	filename += ("." + extension)
	return filename
}

func sendFile(file []byte, filename string, w http.ResponseWriter) error {

	// Send the picture back
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", http.DetectContentType(file))
	w.Header().Set("Content-Length", strconv.Itoa(len(file)))

	_, err := w.Write(file)
	if err != nil {
		return err
	}

	return nil
}

func tarFiles(images *[]tarBuffer) ([]byte, error) {

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, img := range *images {
		hdr := &tar.Header{
			Name: img.Filename,
			Mode: 0666,
			Size: int64(len(*img.Contents)),
		}
		err := tw.WriteHeader(hdr)
		if err != nil {
			return []byte{}, err
		}

		_, err = tw.Write(*img.Contents)
		if err != nil {
			return []byte{}, err
		}
	}

	err := tw.Close()
	if err != nil {
		return []byte{}, err
	}

	return buf.Bytes(), nil
}

type tarBuffer struct {
	Contents *[]byte
	Filename string
}
