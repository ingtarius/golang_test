package main

import (
	"flag"
	"fmt"
	"github.com/valyala/ybc/bindings/go/ybc"
	"gopkg.in/gographics/imagick.v1/imagick"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	//"github.com/nfnt/resize"
)

var (
	defaultCompressionQuality  = flag.Uint("defaultCompressionQuality", 75, "Default compression quality for images. It may be overrided by compressionQuality parameter")
	listenAddr                 = flag.String("listenAddr", ":8081", "TCP address to listen to")
	maxImageSize               = flag.Int64("maxImageSize", 10*1024*1024, "The maximum image size which can be read from imageUrl")
	maxUpstreamCacheItemsCount = flag.Int("maxCachedImagesCount", 10*1000, "The maximum number of images the resizer can cache from upstream servers. Increase this value for saving more upstream bandwidth")
	maxUpstreamCacheSize       = flag.Int("maxUpstreamCacheSize", 1024, "The maximum total size in MB of images the resizer cache cache from upstream servers. Increase this value for saving more upstream bandwidth")
	upstreamCacheFilename      = flag.String("upstreamCacheFilename", "/var/tmp/cache", "Path to cache file for images loaded from upstream. Leave blank for anonymous non-persistent cache")
)

var (
	upstreamCache ybc.Cacher
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// Забираем переменные, если есть
	flag.Parse()

	// Запускаем imagick и сразу завершим после использования
	// https://github.com/gographics/imagick#initialize-and-terminate
	imagick.Initialize()
	defer imagick.Terminate()

	// Работаем с кешом. Непонятная хрень пока что
	upstreamCache = openUpstreamCache()
	defer upstreamCache.Close()

	// Запускаем веб-сервер и ловим возможные ошибки
	if err := http.ListenAndServe(*listenAddr, http.HandlerFunc(serveHTTP)); err != nil {
		logFatal("Error when starting or running http server: %v", err)
	}
}

func Processing(w http.ResponseWriter, r *http.Request, imageUrl string, width, height uint) {
	// Получаем сырую картинку
	blob := getImageBlob(r, imageUrl)
	if blob == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	w.Header().Del("Content-Length")

	if err := mw.ReadImageBlob(blob); err != nil {
		if !strings.HasSuffix(imageUrl, ".ico") {
			logRequestError(r, "Cannot parse image from imageUrl=%v: %v", imageUrl, err)
			// return skipped intentionally
		}
		if _, err = w.Write(blob); err != nil {
			logRequestError(r, "Cannot send image from imageUrl=%v to client: %v", imageUrl, err)
		}
		return
	}
	// Ресайзим
	if err := mw.ThumbnailImage(width, height); err != nil {
		logRequestError(r, "Error when thumbnailing the image obtained from imageUrl=%v: %v", imageUrl, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	//resize.Resize(width, height uint, img image.Image, interp resize.InterpolationFunction) image.Image
	//return
	mw.StripImage()

	if !sendResponse(w, r, mw, imageUrl) {
		// w.WriteHeader() is skipped intentionally here, since the response may be already partially created.
		return
	}
	logRequestMessage(r, "SUCCESS")
}

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "/favicon.ico" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Получаем данные о картинке. Новые высоту-ширину и формируем URL для запроса
	imageUrl, width, height := getImageParams(r)
	if len(imageUrl) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	go Processing(w, r, imageUrl, width, height)
}

func sendResponse(w http.ResponseWriter, r *http.Request, mw *imagick.MagickWand, imageUrl string) bool {
	mw.ResetIterator()
	blob := mw.GetImageBlob()
	format := mw.GetImageFormat()
	contentType := fmt.Sprintf("image/%s", strings.ToLower(format))
	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(blob); err != nil {
		logRequestError(r, "Cannot send image from imageUrl=%v to client: %v", imageUrl, err)
		return false
	}
	return true
}

func getImageParams(r *http.Request) (imageUrl string, width, height uint) {
	s := strings.Split(r.URL.Path, "/")
	size, name := s[1], s[2]
	d := strings.Split(size, "x")
	width64, _ := strconv.ParseUint(d[0], 10, 32)
	height64, _ := strconv.ParseUint(d[1], 10, 32)
	width = uint(width64)
	height = uint(height64)
	imageUrl = fmt.Sprintf("http://%s/140x105/%s", r.Host, name)
	return
}

func getImageBlob(r *http.Request, imageUrl string) []byte {
	blob, err := upstreamCache.Get([]byte(imageUrl))
	if err == nil {
		logRequestMessage(r, "HIT")
		return blob
	}

	if err != ybc.ErrCacheMiss {
		logFatal("Unexpected error when reading data from upstream cache under the key=%v: %v", imageUrl, err)
	}
	logRequestMessage(r, "MISS")
	resp, err := http.Get(imageUrl)
	if err != nil {
		logRequestError(r, "Cannot load image from imageUrl=%v: %v", imageUrl, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logRequestError(r, "Unexpected StatusCode=%d returned from imageUrl=%v", resp.StatusCode, imageUrl)
		return nil
	}
	if blob, err = ioutil.ReadAll(io.LimitReader(resp.Body, *maxImageSize)); err != nil {
		logRequestError(r, "Error when reading image body from imageUrl=%v: %v", imageUrl, err)
		return nil
	}

	if err = upstreamCache.Set([]byte(imageUrl), blob, ybc.MaxTtl); err != nil {
		if err == ybc.ErrNoSpace {
			logRequestError(r, "No enough space for storing image obtained from imageUrl=%v into upstream cache", imageUrl)
		} else {
			logFatal("Unexpected error when storing image under the key=%v in upstream cache: %v", imageUrl, err)
		}
	}
	return blob
}

func openUpstreamCache() ybc.Cacher {
	// https://godoc.org/github.com/valyala/ybc/bindings/go/ybc#Config
	// Посмотреть на понятие горячего кэша в конфиге выше
	config := ybc.Config{
		MaxItemsCount: ybc.SizeT(*maxUpstreamCacheItemsCount),
		DataFileSize:  ybc.SizeT(*maxUpstreamCacheSize) * ybc.SizeT(1024*1024*4),
	}

	var err error
	var cache ybc.Cacher

	if *upstreamCacheFilename != "" {
		config.DataFile = *upstreamCacheFilename + ".cdn-example.data"
		config.IndexFile = *upstreamCacheFilename + ".cdn-example.index"
	}
	cache, err = config.OpenCache(true)
	if err != nil {
		logFatal("Cannot open cache for upstream images: %v", err)
	}
	return cache
}

func logRequestError(r *http.Request, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logRequestMessage(r, "ERROR: %s", msg)
}

func logRequestMessage(r *http.Request, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logMessage("%s - %s - %s - %s. %s", r.RemoteAddr, r.RequestURI, r.Referer(), r.UserAgent(), msg)
}

func logMessage(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s\n", msg)
}

func logFatal(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Fatalf("%s\n", msg)
}
