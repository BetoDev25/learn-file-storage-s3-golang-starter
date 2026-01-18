package main

import (
	"fmt"
	"net/http"
	"strings"
	"io"
	"errors"
	"database/sql"
	"os"
	"path/filepath"
	"mime"
	"encoding/base64"
	"crypto/rand"


	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	//validate the request
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	//get image data from form
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	contentType := header.Header.Get("Content-Type")
	contentType = strings.Split(strings.TrimSpace(contentType), ";")[0]
	fileExtension := strings.Split(contentType, "/")[1]

	mediaType, _, err := mime.ParseMediaType(contentType)
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "thumbnail not jpg or png", err)
		return
	}

	defer file.Close()

	//end of my code

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	//my code
	data := make([]byte, 32)
	_, err = rand.Read(data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't generate random key", err)
		return
	}
	encoded := base64.RawURLEncoding.EncodeToString(data)

	fileName := encoded + "." + fileExtension
	filePath := filepath.Join(cfg.assetsRoot, fileName)
	browserURL := "http://localhost:" + cfg.port + "/" + filePath

	dst, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save file", err)
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
		respondWithError(w, http.StatusUnauthorized, "User does not own video", nil)
		return
	}

	metadata.ThumbnailURL = &browserURL
	fmt.Println("DEBUG metadata.ThumbnailURL:", *metadata.ThumbnailURL)

	err = cfg.db.UpdateVideo(metadata)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't update video", err)
		return
	}

	//end of my code


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here

	respondWithJSON(w, http.StatusOK, metadata)
}

