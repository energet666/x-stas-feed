package media

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func createUniqueFile(root, originalName string) (string, *os.File, error) {
	extension := filepath.Ext(originalName)
	base := strings.TrimSuffix(originalName, extension)
	for attempt := 0; attempt < 10_000; attempt++ {
		filename := originalName
		if attempt > 0 {
			filename = fmt.Sprintf("%s (%d)%s", base, attempt, extension)
		}
		path := filepath.Join(root, filename)
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", nil, err
		}
		if absPath != root && !strings.HasPrefix(absPath, root+string(os.PathSeparator)) {
			return "", nil, errors.New("path escapes root")
		}

		file, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return filename, file, err
	}

	return "", nil, errors.New("could not allocate unique filename")
}
