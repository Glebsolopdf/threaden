package avatar

import (
	"bytes"
	"errors"
	"io"
	"net/http"
)

const maxProfileFieldBytes = 1 << 10

type ProfileParts struct {
	DisplayName string
	Avatar      string
	HasAvatar   bool
}

func ReadMultipartProfile(r *http.Request) (ProfileParts, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return ProfileParts{}, uploadError("avatar upload is invalid")
	}

	var parts ProfileParts
	hasDisplayName := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return parts, nil
		}
		if err != nil {
			return ProfileParts{}, uploadError("avatar upload is invalid")
		}
		switch part.FormName() {
		case "display_name":
			value, err := io.ReadAll(io.LimitReader(part, maxProfileFieldBytes+1))
			if err != nil || len(value) > maxProfileFieldBytes {
				return ProfileParts{}, uploadError("avatar upload is invalid")
			}
			if !hasDisplayName {
				parts.DisplayName = string(value)
				hasDisplayName = true
			}
		case "avatar":
			if parts.HasAvatar {
				return ProfileParts{}, uploadError("avatar upload is invalid")
			}
			value, err := io.ReadAll(io.LimitReader(part, MaxUploadBytes+1))
			if err != nil {
				return ProfileParts{}, uploadError("avatar file is invalid")
			}
			parts.Avatar, err = ProcessUpload(bytes.NewReader(value), int64(len(value)))
			if err != nil {
				return ProfileParts{}, err
			}
			parts.HasAvatar = true
		}
	}
}
