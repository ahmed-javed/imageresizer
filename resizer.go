package imageresizer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"

	"strings"
	"time"

	"gopkg.in/gographics/imagick.v3/imagick"
)

const tmpDirectory = "/tmp/"

//ProductImage exported
type ProductImage struct {
	Angle            string
	Quality          string
	KeepAspectRatio  bool
	KeepFrame        bool
	KeepTransparency bool
	ConstrainOnly    bool
	Width            string
	Height           string
	BackgroundColor  []int
	RemoveBorder     bool
}

//Resize functions read all the resizing info from mongodb, it just takes the valid image url to process
//entity entities.Product,
func ResizeAndUpload(s ProductImage, s3Url, newImgPath, imgName, copyRight, moveOriginalPrefix string, s3c S3Credentials) (p string) {
	cr := strings.Replace(copyRight, "YEAR", strconv.Itoa(time.Now().Year()), 1)

	if imgName == "" {
		_, imgName = saveS3ImageToTmp(s3Url)
	}

	if newImgPath == "" {
		newImgPath, _ = saveS3ImageToTmp(s3Url)
	}

	tmpImg := tmpDirectory + imgName

	imgSize := s.Width + "x" + s.Height
	tmpOutputImg := tmpDirectory + imgSize + "/" + imgName

	err := os.MkdirAll(tmpDirectory+imgSize, os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
	}

	file, e := os.Create(tmpOutputImg)
	f, _ := OsFileExists(tmpOutputImg)
	if !f {
		CheckImageException(e, "file not found "+tmpOutputImg)
	}
	CheckImageException(e)

	//###########Convert Image Logic##############
	//keepAspectRatio - we dont need to set aspect ratio, as image magick always keep the aspect ratio and same is for the website
	//keepFrame - we are not keeping border frames in images on the website so no need to add checks for it
	cmd := []string{
		"mogrify", //basic instruction to convert image
		tmpImg,    //original image path
	}

	if s.Width != "" && s.Height != "" {
		cmd = append(cmd, "-resize", imgSize+">")
	}

	if s.KeepTransparency && s.RemoveBorder == false {
		cmd = append(cmd, "-transparent", "white")
	}

	quality := s.Quality

	if quality == "" {
		quality = "60"
	}

	if s.RemoveBorder == false {
		cmd = append(cmd, "-gravity", "center")
		cmd = append(cmd, "-background", "white")
		if s.Width != "" && s.Height != "" {
			cmd = append(cmd, "-extent", imgSize)
		}
	}

	cmd = append(cmd, "-quality", quality)  //equals to GD 90
	cmd = append(cmd, "-interlace", "line") //progressive images
	cmd = append(cmd, "-unsharp", "0.25x0.08+8.3+0.045")
	cmd = append(cmd, "-gaussian-blur", "0.05")
	cmd = append(cmd, "-sampling-factor", "4:2:0")
	cmd = append(cmd, "-colorspace", "RGB")
	cmd = append(cmd, "-dither", "none")
	cmd = append(cmd, "-define", "jpeg:fancy-upsampling=off")
	cmd = append(cmd, "-define", "png:compression-filter=5")
	cmd = append(cmd, "-define", "png:compression-level=9")
	cmd = append(cmd, "-define", "png:compression-strategy=1")
	cmd = append(cmd, "-define", "png:exclude-chunk=all")
	cmd = append(cmd, "-strip", tmpOutputImg) //output image

	fmt.Println()

	OsFileExists(tmpOutputImg)
	if !f {
		CheckImageException(err, "file not found "+tmpOutputImg)
	}

	imagick.Initialize()
	_, err = imagick.ConvertImageCommand(cmd)

	if err != nil {
		CheckImageException(err, strings.Join(cmd, "  "))
		imagick.Terminate()
		file.Close()
		return
	}

	mwc := imagick.NewMagickWand()
	mwc.ReadImage(tmpOutputImg)
	mwc.CommentImage(cr)
	mwc.WriteImage(tmpOutputImg)
	mwc.Destroy()

	//Close the File
	file.Close()

	// Schedule cleanup
	imagick.Terminate()
	//#########Image Conversion End#####################

	//Return if any error found
	if err != nil {
		return
	}

	//Upload Image
	path := strings.TrimLeft(newImgPath, "/") + "/" + imgName
	if moveOriginalPrefix != "" {
		orgPath := strings.TrimLeft(newImgPath, "/") + moveOriginalPrefix + "/" + imgName
		_ = uploadImageToS3(s3c, tmpImg, orgPath, s)
	}
	p = uploadImageToS3(s3c, tmpOutputImg, path, s)

	return
}

func saveS3ImageToTmp(s string) (dir, filename string) {
	dir, filename = GetFilenameFromPath(s)

	// don't worry about errors
	response, e := http.Get(s)
	if e != nil {
		fmt.Println(e.Error())
	}
	defer response.Body.Close()
	//open a file for writing
	file, e := os.Create(tmpDirectory + filename)
	f, _ := OsFileExists(tmpDirectory + filename)
	if !f {
		panic("file not found " + tmpDirectory + filename)
	}
	if e != nil {
		fmt.Println(e.Error())
	}
	// Use io.Copy to just dump the response body to the file. This supports huge files
	_, e = io.Copy(file, response.Body)
	file.Close()
	if e != nil {
		fmt.Println(e.Error())
	}
	return
}

func GetFilenameFromPath(url string) (dir, file string) {
	rs, e := http.Get(url)
	if e != nil {
		panic(e)
	}
	dir, file = path.Split(rs.Request.URL.Path)
	return
}

func uploadImageToS3(s3c S3Credentials, fileTmpPath, path string, p ProductImage) string {
	s := Upload(s3c, fileTmpPath, path)
	if s == "" {
		fmt.Println(s, " Not Uploaded")
	}
	return s
}

//OsFileExists exported
func OsFileExists(filename string) (bool, bool) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, false
	}
	return !info.IsDir(), info.IsDir()
}

// CheckImageException to check errors
func CheckImageException(err error, info ...string) {
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}
}
