package main

import (
	"bytes"
	"fmt"
	"encoding/json"
	"math"
	"os/exec"
)

func  getVideoAspectRatio(filePath string) (string, error) {
	var buf bytes.Buffer
	command := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	command.Stdout = &buf
	err := command.Run()
	if err != nil {
		fmt.Println("ffprobe command failed with error:", err)
		fmt.Println("ffprobe output:", buf.String())
		return "", err
	}

	type FFprobeStream struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	type FFprobeOutput struct {
		Streams []FFprobeStream `json:"streams"`
	}
	var ffprobeData FFprobeOutput

	if err := json.Unmarshal(buf.Bytes(), &ffprobeData); err != nil {
		return "", err
	}
	tolerance := 0.05
	width := float64(ffprobeData.Streams[0].Width)
	height := float64(ffprobeData.Streams[0].Height)
	diff169 := math.Abs((width/height) - (16.0/9.0))
	diff916 := math.Abs((width/height) - (9.0/16.0))
	ratio := "other"
	if diff169 < diff916 && diff169 < tolerance {
		ratio = "16:9"
	} else if diff916 < diff169 && diff916 < tolerance {
		ratio = "9:16"
	} else {
		ratio = "other"
	}

	return ratio, nil
}

