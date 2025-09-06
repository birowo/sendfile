package sendfile

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/birowo/pool"
	"github.com/birowo/zstdfile"
	"github.com/valyala/fasthttp"
)

var Mime = map[string]string{".html": "text/html; charset=utf-8"}

func SendFile(name string, poolSize int32) fasthttp.RequestHandler {
	zsname := zstdfile.Name(name)
	zstdfile.Encode(name, zsname)
	type F struct {
		f, zf  *os.File
		lm, ct string
		error
	}
	ct := Mime[name[strings.LastIndexByte(name, '.'):]]
	fp := pool.New(poolSize, func() (f F) {
		f.f, f.error = os.Open(name)
		if f.error != nil {
			return
		}
		f.zf, f.error = os.Open(zsname)
		if f.error != nil {
			return
		}
		i, _ := f.f.Stat()
		f.lm = i.ModTime().Format(http.TimeFormat)
		println(i.Size())
		f.ct = ct
		return
	})
	return func(ctx *fasthttp.RequestCtx) {
		f := fp.Get()
		if f.error != nil {
			log.Println(f.error)
			return
		}
		defer func() {
			if !fp.Put(f) {
				f.f.Close()
				f.zf.Close()
			}
		}()
		if string(ctx.Request.Header.Peek("If-Modified-Since")) == f.lm {
			ctx.SetStatusCode(fasthttp.StatusNotModified)
			return
		}
		ctx.SetContentType(f.ct)
		ctx.Response.Header.Set("Last-Modified", f.lm)
		if bytes.Contains(ctx.Request.Header.Peek("Accept-Encoding"), []byte(zstdfile.Path)) {
			ctx.Response.Header.Set("Content-Encoding", zstdfile.Path)
			f.zf.Seek(0, 0)
			f.zf.WriteTo(ctx)
			return
		}
		f.f.Seek(0, 0)
		f.f.WriteTo(ctx)
	}
}
