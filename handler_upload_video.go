package main

import (
	"net/http"
	"fmt"
	"database/sql"
	"errors"
	"path"
	"mime"
	"os"
	"io"
	"strings"
	"crypto/rand"
	"context"
	"encoding/hex"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, int64(uploadLimit))

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get token", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "video does not exist", err)
			return
		} else {
			respondWithError(w, http.StatusBadRequest, "Couldn't get video", err)
			return
		}
	}
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "you do not own this video", nil)
		return
	}

	uploadedVideo, header, err := r.FormFile("video")
	defer uploadedVideo.Close()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse video", err)
		return
	}

	contentType := header.Header.Get("Content-Type")
	contentType = strings.Split(strings.TrimSpace(contentType), ";")[0]

	parsedType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if parsedType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "video not mp4", nil)
		return
	}

	//create and copy file
	temp, err := os.CreateTemp("", "tubely-upload.mp4")
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't create temp file", err)
		return
	}

	_, err = io.Copy(temp, uploadedVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save upload", err)
		return
	}

	_, err = temp.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't reset temp file", err)
		return
	}

	ratio, err := getVideoAspectRatio(temp.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get aspect ratio", err)
		return
	}
	//aspect ratio
	aspect := ""
	if ratio == "16:9" {
		aspect = "landscape"
	} else if ratio == "9:16" {
		aspect = "portrait"
	} else {
		aspect = "other"
	}

	//putting the object
	raw := make([]byte, 32)
	_, err = rand.Read(raw)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't generate random key", err)
		return
	}

	key0 := hex.EncodeToString(raw) + ".mp4"
	key := path.Join(aspect, key0)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        temp,
		ContentType: aws.String(parsedType),
	}

	_, err = cfg.s3Client.PutObject(context.Background(), input)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't store file in S3 bucket", err)
		return
	}

	url := "https://" +  cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + key
	fmt.Println("Generated URL:", url)
	metadata.VideoURL = &url
	err = cfg.db.UpdateVideo(metadata)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't update video", err)
		return
	}

}
