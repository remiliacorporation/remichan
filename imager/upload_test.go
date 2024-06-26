package imager

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bakape/meguca/common"
	"github.com/bakape/meguca/config"
	"github.com/bakape/meguca/db"
	"github.com/bakape/meguca/imager/assets"
	"github.com/bakape/meguca/test"
	"github.com/bakape/meguca/test/test_db"
)

func newMultiWriter() (*bytes.Buffer, *multipart.Writer) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	return body, writer
}

func newRequest(
	t *testing.T,
	body io.Reader,
	w *multipart.Writer,
) *http.Request {
	t.Helper()

	req := httptest.NewRequest("PUT", "/", body)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func setHeaders(req *http.Request, headers map[string]string) {
	for key, val := range headers {
		req.Header.Set(key, val)
	}
}

func assertCode(t *testing.T, res, std int) {
	t.Helper()
	if res != std {
		t.Errorf("unexpected status code: %d : %d", std, res)
	}
}

func newJPEGRequest(t *testing.T) *http.Request {
	t.Helper()

	b, w := newMultiWriter()

	file, err := w.CreateFormFile("image", assets.StdJPEG.Name)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write(test.ReadSample(t, assets.StdJPEG.Name))
	if err != nil {
		t.Fatal(err)
	}

	req := newRequest(t, b, w)
	req.Header.Set("Content-Length", "300792")

	return req
}

func getImageRecord(t *testing.T, id string) common.ImageCommon {
	t.Helper()

	img, err := db.GetImage(id)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// Assert image file assets were created with the correct paths
func assertFiles(t *testing.T, src, id string, fileType, thumbType uint8) {
	t.Helper()

	var (
		paths [3]string
		data  [3][]byte
	)
	paths[0] = filepath.Join("testdata", src)
	destPaths := assets.GetFilePaths(id, fileType, thumbType)
	paths[1], paths[2] = destPaths[0], destPaths[1]

	for i := range paths {
		var err error
		data[i], err = ioutil.ReadFile(paths[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	test.AssertBufferEquals(t, data[0], data[1])
	if len(data[1]) < len(data[2]) {
		t.Error("unexpected file size difference")
	}
}

func TestInvalidContentLengthHeader(t *testing.T) {
	b, w := newMultiWriter()
	req := newRequest(t, b, w)
	setHeaders(req, map[string]string{
		"Content-Length": "KAWFEE",
	})

	_, err := ParseUpload(req)
	if s := fmt.Sprint(err); !strings.Contains(s, "invalid syntax") {
		test.UnexpectedError(t, err)
	}
}

func TestUploadTooLarge(t *testing.T) {
	conf := config.Get()
	(*conf).MaxSize = 1
	b, w := newMultiWriter()
	req := newRequest(t, b, w)
	req.Header.Set("Content-Length", "1048577")

	_, err := ParseUpload(req)
	test.AssertEquals(t, common.StatusError{errTooLarge, 400}, err)
}

func TestInvalidForm(t *testing.T) {
	b, w := newMultiWriter()
	req := newRequest(t, b, w)
	setHeaders(req, map[string]string{
		"Content-Length": "1024",
		"Content-Type":   "GWEEN TEA",
	})

	if _, err := ParseUpload(req); err == nil {
		t.Fatal("expected an error")
	}
}

func TestNewThumbnail(t *testing.T) {
	test_db.ClearTables(t, "images")
	resetDirs(t)
	config.Set(config.Configs{
		Public: config.Public{
			MaxSize: 10,
		},
	})

	req := newJPEGRequest(t)
	rec := httptest.NewRecorder()
	NewImageUpload(rec, req)
	assertCode(t, rec.Code, 200)

	img := getImageRecord(t, assets.StdJPEG.SHA1)
	test.AssertEquals(t, img, assets.StdJPEG.ImageCommon)
	assertFiles(t, "sample.jpg", assets.StdJPEG.SHA1, common.JPEG, common.WEBP)
}

func TestNoImageUploaded(t *testing.T) {
	b, w := newMultiWriter()
	req := newRequest(t, b, w)
	req.Header.Set("Content-Length", "300792")

	_, err := ParseUpload(req)
	test.AssertEquals(t, common.StatusError{http.ErrMissingFile, 400}, err)
}

func TestThumbNailReuse(t *testing.T) {
	test_db.ClearTables(t, "images")
	resetDirs(t)

	for i := 1; i <= 2; i++ {
		req := newJPEGRequest(t)
		_, err := ParseUpload(req)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestUploadImageHash(t *testing.T) {
	test_db.ClearTables(t, "images")
	resetDirs(t)

	std := assets.StdJPEG

	req := newJPEGRequest(t)
	_, err := ParseUpload(req)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	b := bytes.NewReader([]byte(std.SHA1))
	req = httptest.NewRequest("POST", "/", b)
	UploadImageHash(rec, req)
	if rec.Code != 200 {
		t.Errorf("unexpected status code: %d", rec.Code)
	}
}

func TestUploadImageHashNoHash(t *testing.T) {
	test_db.ClearTables(t, "images")

	rec := httptest.NewRecorder()
	b := bytes.NewReader([]byte(assets.StdJPEG.SHA1))
	req := httptest.NewRequest("POST", "/", b)
	UploadImageHash(rec, req)
	if rec.Code != 200 {
		t.Errorf("unexpected status code: %d", rec.Code)
	}
	if s := rec.Body.String(); s != "" {
		t.Errorf("unexpected response body: `%s`", s)
	}
}
