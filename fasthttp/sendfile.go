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
	}
	ct := Mime[name[strings.LastIndexByte(name, '.'):]]
	fp := pool.New(poolSize, func() (f F) {
		var err error
		f.f, err = os.Open(name)
		if err != nil {
			log.Println(err)
			return
		}
		f.zf, err = os.Open(zsname)
		if err != nil {
			log.Println(err)
			return
		}
		info, _ := f.f.Stat()
		f.lm = info.ModTime().Format(http.TimeFormat)
		f.ct = ct
		return
	}, func(f F) {
		f.f.Close()
		f.zf.Close()
	})
	return func(ctx *fasthttp.RequestCtx) {
		f := fp.Get()
		if f.zf != nil {
			return
		}
		defer fp.Put(f)
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
