package imageresizer

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	goamzAWS "github.com/AdRoll/goamz/aws"
	goamzS3 "github.com/AdRoll/goamz/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var uploader *s3manager.Uploader

var sess *session.Session

const maxAge = "max-age=172800"

type S3Credentials struct {
	Key    string
	Secret string
	Region string
	Bucket string
}

var s3c S3Credentials

func s3Setup(s S3Credentials) {
	s3c = s
	// Create a single AWS session (we can re use this if we're uploading many files)
	creds := credentials.NewStaticCredentials(s.Key, s.Secret, "")
	sess, err := session.NewSession(
		&aws.Config{
			Region:      aws.String(s.Region),
			Credentials: creds,
		},
	)

	if err != nil {
		fmt.Println(err.Error())
	}

	// S3 service client the Upload manager will use.
	s3Svc := s3.New(sess)

	// Create an uploader with S3 client and custom options
	uploader = s3manager.NewUploaderWithClient(s3Svc)
}

//Upload exported
func Upload(s S3Credentials, tmpFilePath, s3FilePath string) string {
	if s3c.Key == "" {
		s3Setup(s)
	}

	r, err := UploadFileToS3(tmpFilePath, s3FilePath)
	if err != nil {
		fmt.Println(err.Error())
	}
	return r
}

// UploadFileToS3 exported
func UploadFileToS3(tmpFilePath, s3FilePath string) (string, error) {
	// Open the file for use
	file, err := os.Open(tmpFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Get file size and read the file content into a buffer
	fileInfo, _ := file.Stat()
	size := fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)

	l, err := pushToS3(size, s3FilePath, &buffer)

	return l, err
}

func pushToS3(fileSize int64, s3FilePath string, data *[]byte) (string, error) {
	var fileType = "image/jpeg"
	if strings.Contains(s3FilePath, "png") {
		fileType = "image/png"
	}
	exp := time.Now()
	exp = exp.Add(time.Hour * 48)

	// Upload input parameters
	upParams := &s3manager.UploadInput{
		Bucket:       aws.String(s3c.Bucket),
		Key:          aws.String(s3FilePath),
		Body:         bytes.NewReader(*data),
		ContentType:  aws.String(fileType),
		CacheControl: aws.String(maxAge),
		Expires:      &exp,
	}

	// Perform an upload.
	r, err := uploader.Upload(upParams)
	l := ""
	if err != nil {
		fmt.Println("S3 File Upload Error ", err.Error())
	} else {
		l = r.Location
	}

	if err != nil {
		fmt.Println("trying with PutObjectInput ", err.Error())
		p, e := s3.New(sess).PutObject(&s3.PutObjectInput{
			Bucket:       aws.String(s3c.Bucket),
			Key:          aws.String(s3FilePath),
			Body:         bytes.NewReader(*data),
			ContentType:  aws.String(fileType),
			CacheControl: aws.String(maxAge),
			Expires:      &exp,
		})
		l = p.GoString()
		err = e
	}

	return l, err
}

func fileExists(filePath string) (exists bool) {
	auth, e := goamzAWS.GetAuth(s3c.Key, s3c.Secret, "", time.Now().Add(time.Minute+2))
	if e != nil {
		fmt.Println(e.Error())
	}
	//change the signature to new v4signature in the s3.go file in the package
	S3 := goamzS3.New(auth, goamzAWS.GetRegion(s3c.Region))
	b := S3.Bucket(s3c.Bucket)
	exists, e = b.Exists(filePath)
	if e != nil {
		fmt.Println(e.Error())
	}
	return
}
