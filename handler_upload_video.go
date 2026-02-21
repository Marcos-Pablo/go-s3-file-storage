package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit = 1 << 30

	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)
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

	video, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unable to update this video", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse form file", err)
		return
	}

	file, header, err := r.FormFile("video")

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type header", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	tmpFile, err := os.CreateTemp("", "tubely-upload.mp4")

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create temporary file", err)
		return
	}

	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, file)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to write to temporary file", err)
		return
	}

	_, err = tmpFile.Seek(0, io.SeekStart)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to reset temporary file pointer", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tmpFile.Name())

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video aspect ratio", err)
		return
	}

	key := path.Join(aspectRatio, getAssetPath(mediaType))

	cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        tmpFile,
		ContentType: &mediaType,
	})

	URL := cfg.getS3objectURL(key)
	video.VideoURL = &URL

	err = cfg.db.UpdateVideo(video)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
