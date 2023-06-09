package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sync/errgroup"
	"google.golang.org/api/slides/v1"
)

var (
	flagFormat      string
	flagOutput      string
	flagName        string
	flagJPEGQuality int
)

func init() {
	flag.StringVar(&flagFormat, "format", "png", "image format(png|jpeg)")
	flag.StringVar(&flagOutput, "output", ".", "path for output directory")
	flag.StringVar(&flagName, "name", "slide%40d.%s", "file name format")
	flag.IntVar(&flagJPEGQuality, "jpeg-quality", 100, "Quality of JEPG image")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	if err := run(ctx, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {

	if len(args) <= 0 {
		return errors.New("presen id must be specified")
	}

	s, err := slides.NewService(ctx)
	if err != nil {
		return err
	}

	id := args[0]
	p, err := s.Presentations.Get(id).Context(ctx).Do()
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.GOMAXPROCS(0))
	for i, page := range p.Slides {
		i, page := i, page
		if page.SlideProperties.IsSkipped {
			continue
		}

		eg.Go(func() error {
			return saveThumnail(ctx, s, i+1, id, page.ObjectId)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func saveThumnail(ctx context.Context, s *slides.Service, no int, presenID, pageID string) error {
	t, err := s.Presentations.Pages.GetThumbnail(presenID, pageID).Context(ctx).Do()
	if err != nil {
		return err
	}

	resp, err := http.Get(t.ContentUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return err
	}

	switch flagFormat {
	case "png":
		fname := filepath.Join(flagOutput, fmt.Sprintf(flagName, no, "png"))
		file, err := os.Create(fname)
		if err != nil {
			return err
		}
		defer file.Close()

		if err := png.Encode(file, img); err != nil {
			return err
		}

		if err := file.Sync(); err != nil {
			return err
		}

		fmt.Println("create", fname)
	case "jpeg", "jpg":
		fname := filepath.Join(flagOutput, fmt.Sprintf(flagName, no, "jpg"))
		file, err := os.Create(fname)
		if err != nil {
			return err
		}
		defer file.Close()

		opt := &jpeg.Options{Quality: flagJPEGQuality}
		if err := jpeg.Encode(file, img, opt); err != nil {
			return err
		}

		if err := file.Sync(); err != nil {
			return err
		}

		fmt.Println("create", fname)
	}

	return nil
}
