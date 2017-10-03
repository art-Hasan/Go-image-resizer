package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/nfnt/resize"
)

type ResizedImage struct {
	Filename string
	Ext      string
	Width    uint
	Height   uint
	Data     *image.Image
}

// Channel for store resized images.
var resized = make(chan ResizedImage)

// Check slice contains string.
func StringInSlice(s string, list []string) bool {
	for _, b := range list {
		if b == s {
			return true
		}
	}
	return false
}

func GetNewWidth(w int, sc int) uint {
	var nw uint
	if sc < 0 {
		nw = uint(-w / sc)
	} else {
		nw = uint(w + w/sc)
	}
	return nw
}

func GetNewHeight(h int, sc int) uint {
	var nh uint
	if sc < 0 {
		nh = uint(-h / sc)
	} else {
		nh = uint(h + h/sc)
	}
	return nh
}

// Get all image files and save to passed buffer.
func GetImageFiles(dirname string, recurs bool, result *[]string) {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatalf("Error: %v", err)
		os.Exit(1)
	}

	// list of provided image extensions
	providedExt := []string{".jpg", ".jpeg", ".png"}
	for _, file := range files {
		// configure path to file
		absPath := filepath.Join(dirname, file.Name())
		ext := filepath.Ext(absPath)

		if recurs {
			if file.IsDir() {
				GetImageFiles(absPath, recurs, result)
			} else {
				// Check is file extension provided.
				if StringInSlice(ext, providedExt) {
					// Add path to file to result slice.
					*result = append(*result, absPath)
				}
			}
		} else {
			if !file.IsDir() {
				if StringInSlice(ext, providedExt) {
					*result = append(*result, absPath)
				}
			}
		}
	}
}

// Parse command line arguments and return flag values.
func ParseCommandLine() (bool, string, int, string) {
	// configure command line flags
	recursively := flag.Bool("r", false, "Sets for recursively obtain all images in directory.")
	imgPath := flag.String("d", "", "Sets the path to dir containing images.")
	scaling := flag.Int("sc", 1, "Set x times scaling value. Negative means downscaling. Positive upscaling.")
	pathToSave := flag.String("p", *imgPath, "Sets the path to save resized images. Default value -d flag value.")

	// parse command line flags
	flag.Parse()

	// return flag values
	return *recursively, *imgPath, *scaling, *pathToSave
}

func Resize(files []string, scale int, wg *sync.WaitGroup) {

	var resizedImg ResizedImage

	for _, filename := range files {

		// open file.
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("97: Error: %v", err)
		}

		ext := filepath.Ext(filename)
		// decode jpeg into image.Image
		if ext == ".jpeg" || ext == ".jpg" {
			img, err := jpeg.Decode(file)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			// Get image size.
			width := GetNewWidth(img.Bounds().Max.X, scale)
			height := GetNewHeight(img.Bounds().Max.Y, scale)

			// resize using Lanczos resampling and preserve aspect ratio
			m := resize.Resize(width, height, img, resize.Lanczos3)
			resizedImg = ResizedImage{filename, ext, width, height, &m}

			// decode png into image.Image
		} else if ext == ".png" {
			img, err := png.Decode(file)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}

			// Get image size.
			width := GetNewWidth(img.Bounds().Max.X, scale)
			height := GetNewHeight(img.Bounds().Max.Y, scale)

			// resize using Lanczos resampling and preserve aspect ratio
			m := resize.Resize(width, height, img, resize.Lanczos3)
			resizedImg = ResizedImage{filename, ext, width, height, &m}
		}
		defer file.Close()

		// send filename and resized image data to channel
		resized <- resizedImg
	}
	// Decrement the counter when goroutine completes.
	defer wg.Done()
}

func HandleResized(savedir string, n int, wg *sync.WaitGroup) {

	var (
		err error
		out *os.File
	)

	// Create savedir if not exist.
	if _, e := os.Stat(savedir); os.IsNotExist(e) {
		// Create directory with ModePerm = 0777 permissions.
		os.Mkdir(savedir, os.ModePerm)
	}

	for i := 0; i < n; i++ {
		// Get data from channel.
		resizedImg := <-resized

		// Change initial image.
		if savedir == "" {
			out, err = os.Create(resizedImg.Filename)
		} else {
			// Get image size.
			width := resizedImg.Width
			height := resizedImg.Height

			// Get file name from full path.
			_, filename := filepath.Split(resizedImg.Filename)

			// Configure new file name.
			newFilename := fmt.Sprint(width) + "x" + fmt.Sprint(height) + "_" + filename
			pathToSave := filepath.Join(savedir, newFilename)

			// Create new file.
			out, err = os.Create(pathToSave)
		}
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		defer out.Close()

		// Write resized image to newly created file.
		if resizedImg.Ext == ".jpeg" || resizedImg.Ext == ".jpg" {
			jpeg.Encode(out, *resizedImg.Data, nil)
		} else if resizedImg.Ext == ".png" {
			png.Encode(out, *resizedImg.Data)
		}
	}
	// Decrement the counter when goroutine completes.
	defer wg.Done()
}

func main() {
	// Get flag values.
	recFlag, pathFlag, scaleFlag, savePath := ParseCommandLine()
	if pathFlag == "" {
		log.Fatalf("Error. Path to directory containing images required.")
	}
	if scaleFlag == 0 {
		log.Fatalf("Error. Scale value should be different from zero.")
	}

	var files []string
	// Get all image files and save it to files buffer.
	GetImageFiles(pathFlag, recFlag, &files)

	var wg sync.WaitGroup
	// Increment the WaitGroup counter.
	wg.Add(1)
	// Resize images.
	go Resize(files, scaleFlag, &wg)

	// Increment the WaitGroup counter.
	wg.Add(1)
	// Change name resized images and save to specific path.
	go HandleResized(savePath, len(files), &wg)

	// Wait for complete goroutines.
	wg.Wait()

	// Receive a message about completion.
	log.Printf("Done. Resized %v img.", len(files))
}
