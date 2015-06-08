package rapid

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
)

type FileDownload struct {
	Filename  string
	MediaType string
	Reader    io.ReadCloser
}

func (f *FileDownload) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	h := w.Header()
	h.Set("Content-Type", f.MediaType)
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": f.Filename,
	}))
	w.WriteHeader(status)
	_, err = io.Copy(w, f.Reader)
	return err
}

func (f *FileDownload) DecodeResponse(r *http.Response) error {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Disposition"))
	if err != nil {
		return err
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	f.MediaType = mt
	f.Filename = params["filename"]
	f.Reader = r.Body
	return nil
}

type FileUpload struct {
	Filename  string
	MediaType string
	Reader    io.ReadCloser
}

func (f *FileUpload) EncodeRequest() (headers http.Header, body io.ReadCloser, err error) {
	headers = http.Header{}
	headers.Set("Content-Type", f.MediaType)
	headers.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": f.Filename,
	}))
	return headers, f.Reader, nil
}

func (f *FileUpload) DecodeRequest(r *http.Request) error {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Disposition"))
	if err != nil {
		return err
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	f.MediaType = mt
	f.Filename = params["filename"]
	f.Reader = r.Body
	return nil
}

type RawData []byte

func (d *RawData) EncodeRequest() (headers http.Header, body io.ReadCloser, err error) {
	headers = http.Header{}
	headers.Add("Content-Type", "application/octet-stream")
	return headers, ioutil.NopCloser(bytes.NewReader(*d)), nil
}

func (d *RawData) DecodeRequest(r *http.Request) error {
	defer r.Body.Close()
	w := bytes.NewBuffer(nil)
	if _, err := io.Copy(w, r.Body); err != nil {
		return err
	}
	*d = w.Bytes()
	return nil
}

func (d *RawData) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	_, e := io.Copy(w, bytes.NewReader(*d))
	return e
}

func (d *RawData) DecodeResponse(r *http.Response) error {
	defer r.Body.Close()
	w := bytes.NewBuffer(nil)
	if _, err := io.Copy(w, r.Body); err != nil {
		return err
	}
	*d = w.Bytes()
	return nil
}
