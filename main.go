package main

import (
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
	r.ParseMultipartForm(100 << 20)
	format := r.FormValue("format")
	switch format {
	case "png", "jpg", "jpeg", "gif":
		fmt.Println("converting to", format)
		f, fheader, err := r.FormFile("image")
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Error uploading file: %v", err)))
			return
		}
		convImg, err := convert(format, f)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Error converting file: %v", err)))
			return
		}

		// Make a new filename - same as old with new extension
		filename := replaceExt(fheader.Filename, format)

		// Send the picture back
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Type", http.DetectContentType(convImg))
		w.Header().Set("Content-Length", strconv.Itoa(len(convImg)))
		w.Write(convImg)

	case "":
		fmt.Println("didn't specify format")
		w.Write([]byte("Didn't specify format :("))
		return
	default:
		fmt.Println("conv was", format)
		w.Write([]byte(fmt.Sprintf("Don't know how to convert to %s :(", format)))
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
