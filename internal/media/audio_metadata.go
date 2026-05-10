package media

import (
	"encoding/json"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

const maxAudioTagLength = 160

type AudioTags struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"albumArtist,omitempty"`
	Date        string `json:"date,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Track       string `json:"track,omitempty"`
}

type probedMedia struct {
	Kind            string
	DurationSeconds float64
	Tags            AudioTags
	HasCover        bool
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType   string            `json:"codec_type"`
		Disposition map[string]int    `json:"disposition"`
		Tags        map[string]string `json:"tags"`
	} `json:"streams"`
	Format struct {
		Duration string            `json:"duration"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
}

func probeMedia(path string) (probedMedia, error) {
	ffprobe, err := ffprobePath()
	if err != nil {
		return probedMedia{}, err
	}

	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	}
	output, err := exec.Command(ffprobe, args...).Output()
	if err != nil {
		return probedMedia{}, err
	}

	var parsed ffprobeOutput
	if err := json.Unmarshal(output, &parsed); err != nil {
		return probedMedia{}, err
	}

	var hasAudio bool
	var hasVideo bool
	var hasCover bool
	for _, stream := range parsed.Streams {
		switch stream.CodecType {
		case "audio":
			hasAudio = true
		case "video":
			if stream.Disposition["attached_pic"] == 1 {
				hasCover = true
			} else {
				hasVideo = true
			}
		}
	}

	kind := ""
	if hasAudio && !hasVideo {
		kind = "audio"
	} else if hasVideo {
		kind = "video"
	}

	return probedMedia{
		Kind:            kind,
		DurationSeconds: parseProbeDuration(parsed.Format.Duration),
		Tags:            audioTagsFromProbe(parsed),
		HasCover:        hasCover,
	}, nil
}

func audioTagsFromProbe(parsed ffprobeOutput) AudioTags {
	tags := parsed.Format.Tags
	for _, stream := range parsed.Streams {
		if stream.CodecType == "audio" && len(stream.Tags) > 0 {
			tags = mergeProbeTags(tags, stream.Tags)
			break
		}
	}

	return AudioTags{
		Title:       cleanAudioTag(firstProbeTag(tags, "title")),
		Artist:      cleanAudioTag(firstProbeTag(tags, "artist", "artists")),
		Album:       cleanAudioTag(firstProbeTag(tags, "album")),
		AlbumArtist: cleanAudioTag(firstProbeTag(tags, "album_artist", "albumartist", "album artist")),
		Date:        cleanAudioTag(firstProbeTag(tags, "date", "year")),
		Genre:       cleanAudioTag(firstProbeTag(tags, "genre")),
		Track:       cleanAudioTag(firstProbeTag(tags, "track", "tracknumber", "track_number")),
	}
}

func mergeProbeTags(primary, fallback map[string]string) map[string]string {
	if len(primary) == 0 {
		return fallback
	}
	merged := make(map[string]string, len(primary)+len(fallback))
	for key, value := range fallback {
		merged[key] = value
	}
	for key, value := range primary {
		merged[key] = value
	}
	return merged
}

func firstProbeTag(tags map[string]string, names ...string) string {
	for _, name := range names {
		for key, value := range tags {
			if strings.EqualFold(key, name) {
				return value
			}
		}
	}
	return ""
}

func cleanAudioTag(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if len([]rune(value)) <= maxAudioTagLength {
		return value
	}
	return string([]rune(value)[:maxAudioTagLength])
}

func parseProbeDuration(value string) float64 {
	duration, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 {
		return 0
	}
	return math.Round(duration*1000) / 1000
}

func (tags AudioTags) empty() bool {
	return tags.Title == "" &&
		tags.Artist == "" &&
		tags.Album == "" &&
		tags.AlbumArtist == "" &&
		tags.Date == "" &&
		tags.Genre == "" &&
		tags.Track == ""
}
