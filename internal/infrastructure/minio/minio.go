package minio

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/ruziba3vich/argus/internal/pkg/config"
)

const MaxFileSize = 10 * 1024 * 1024

var (
	ErrFileTooLarge     = fmt.Errorf("file size exceeds the maximum limit of %d bytes", MaxFileSize)
	ErrFileNotFound     = fmt.Errorf("file not found")
	ErrInvalidFileType  = fmt.Errorf("invalid file type")
	ErrFileUploadFailed = fmt.Errorf("file upload failed")
	ErrFileDeleteFailed = fmt.Errorf("file deletion failed")
)

type MinIOClient struct {
	Client *minio.Client
	config *config.Config
}

func NewMinIOClient(cfg *config.Config) (*MinIOClient, error) {

	client, err := minio.New(cfg.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("EgT8rW6kmagjSxi58uDI", cfg.MinIO.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		fmt.Println("error creating: ", err)
		return nil, err
	}

	// Create bucket if not exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.MinIO.BucketName)
	if err != nil {
		fmt.Println("err bucket : ", err)
		return nil, err
	}
	if !exists {
		err = client.MakeBucket(ctx, cfg.MinIO.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			fmt.Println("err making bucket : ", err)
			return nil, err
		}
		fmt.Println("Bucket created:", cfg.MinIO.BucketName)
	}

	return &MinIOClient{Client: client, config: cfg}, nil
}

func (m *MinIOClient) UploadFile(file multipart.File, fileName string, fileSize int64, contentType string) (string, error) {
	ctx := context.Background()

	_, err := m.Client.PutObject(ctx, m.config.MinIO.BucketName, fileName, file, fileSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		fmt.Println("err put: ", err)
		return "", err
	}

	fileURL := fmt.Sprintf("http://%s/%s/%s", m.config.MinIO.Endpoint, m.config.MinIO.BucketName, fileName)
	return fileURL, nil
}

func (m *MinIOClient) DeleteFile(fileName string) error {
	ctx := context.Background()

	err := m.Client.RemoveObject(ctx, m.config.MinIO.BucketName, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		fmt.Println("err delete: ", err)
		return err
	}

	fmt.Println("File deleted successfully:", fileName, "----", err)

	return nil
}
