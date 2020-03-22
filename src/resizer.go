package main

import (
	"context"
	"flag"
	"fmt"
	"golang.org/x/sync/errgroup"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/nfnt/resize"
)

const (
	extJpg = ".jpg"
	extJpeg = ".jpeg"
	extPng = ".png"
)

type Image struct {
	Filename string
	Ext      string
	Width    uint
	Height   uint
	Data     image.Image
}

func width(w int, sc int) uint {
	var nw uint
	if sc < 0 {
		nw = uint(-w / sc)
	} else {
		nw = uint(w + w/sc)
	}
	return nw
}

func height(h int, sc int) uint {
	var nh uint
	if sc < 0 {
		nh = uint(-h / sc)
	} else {
		nh = uint(h + h/sc)
	}
	return nh
}

type Resizer struct {
	files []string
	dir string
	saveDir string
	sc int
	recursive bool

	tasks chan Image
}

type ResizerOptions struct {
	Dir string
	SaveDir string
	Scale int
	Recursive bool
}

func NewResizer(ctx context.Context, opt ResizerOptions) (*Resizer, error) {
	r := &Resizer{
		dir: opt.Dir,
		saveDir: opt.SaveDir,
		sc: opt.Scale,
		recursive: opt.Recursive,
		files: make([]string, 0),
		tasks: make(chan Image, 1),
	}
	if err := r.getImages(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Resizer) getImages(ctx context.Context) error {
	fileInfo, err := ioutil.ReadDir(r.dir)
	if err != nil {
		return err
	}

	for _, file := range fileInfo {
		ext := filepath.Ext(file.Name())

		if r.recursive {
			if file.IsDir() {
				if err := r.getImages(ctx); err != nil {
					return err
				}
			} else {
				if ext == extJpg || ext == extJpeg || ext == extPng {
					r.files = append(r.files, filepath.Join(r.dir, file.Name()))
				}
			}
		} else {
			if ext == extJpg || ext == extJpeg || ext == extPng {
				r.files = append(r.files, filepath.Join(r.dir, file.Name()))
			}
		}
	}
	return nil
}

func (r *Resizer) Resize(ctx context.Context) error {
	var (
		img  Image
		source image.Image
		file *os.File

		err error
	)

	for _, filename := range r.files {
		file, err = os.Open(filename)
		if err != nil {
			return err
		}

		ext := filepath.Ext(filename)
		switch {
		case ext == extJpeg || ext == extJpg:
			source, err = jpeg.Decode(file)
			if err != nil {
				return err
			}
		case ext == extPng:
			source, err = png.Decode(file)
			if err != nil {
				return err
			}
		}

		width := width(source.Bounds().Max.X, r.sc)
		height := height(source.Bounds().Max.Y, r.sc)

		m := resize.Resize(width, height, source, resize.Lanczos3)
		img = Image{
			Filename: filename,
			Ext: ext,
			Width: width,
			Height: height,
			Data: m,
		}
		r.tasks <- img
	}
	defer func() {
		_ = file.Close()
		close(r.tasks)
	}()

	return nil
}

func (r *Resizer) Save(ctx context.Context) error {
	var (
		err error
		out *os.File
	)

	if _, err := os.Stat(r.saveDir); os.IsNotExist(err) {
		if err := os.Mkdir(r.saveDir, 0644); err != nil {
			return err
		}
	}

	for img := range r.tasks {
		filename := fmt.Sprintf(
			"%dx%d_%d.%s",
			img.Width, img.Height, time.Since(time.Unix(0, time.Now().Unix())), img.Ext,
		)

		out, err = os.Create(filename)
		if err != nil {
			return err
		}

		switch {
		case img.Ext == extJpeg || img.Ext == extJpg:
			if err := jpeg.Encode(out, img.Data, nil); err != nil {
				return err
			}
		case img.Ext == extPng:
			if err := png.Encode(out, img.Data); err != nil {
				return err
			}
		}
	}
	defer func() {
		_ = out.Close()
	}()

	return nil
}

func main() {
	var (
		r       = flag.Bool("r", false, "Sets for recursively obtain all images in directory.")
		sc      = flag.Int("sc", 1, "Set x times scaling value. Negative means downscaling. Positive up scaling.")
		dir     = flag.String("d", "", "Sets the path to dir containing images.")
		saveDir = flag.String("p", *dir, "Sets the path to save resized images. Default value -d flag value.")
	)
	flag.Parse()

	if *dir == "" {
		log.Fatalf("Path to directory containing images required")
	}
	if *sc == 0 {
		log.Fatalf("Scale value should be different from zero")
	}

	resizer, err := NewResizer(context.Background(), ResizerOptions{
		Dir:       *dir,
		SaveDir:   *saveDir,
		Scale:     *sc,
		Recursive: *r,
	})
	if err != nil {
		log.Fatal(err)
	}

	eg, runCtx := errgroup.WithContext(context.Background())
	eg.Go(func() error { return resizer.Resize(runCtx) })
	go func() {
		if err := eg.Wait(); err != nil {
			log.Fatal(err)
		}
	}()

	if err := resizer.Save(runCtx); err != nil {
		log.Fatal(err)
	}
}
