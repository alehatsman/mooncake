package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// embeddedAnimationFrames contains the mooncake animation frames
const embeddedAnimationFrames = `٩     ۶
(⚆ ◡ ⚆)
 ◡   ◡

 ٩   ۶
(⨀ _ ⨀)
 ◡  ◡

 ⚲    ⚲
(⚆ ◡ ⚆)
  ◡ ◡

 ⚲    ⚲
(⨀ ◡ ⨀)
  ◡  ◡

  ٩   ۶
(⚆ _ ⚆)
 ◡  ◡

 ٩    ۶
(⚆ ⯋ ⚆)
  ◡  ◡

  ۶   ۶
(⚆ ⯋ ⚆)
  ◡  ◡

 ٩    ۶
(⚆ ? ⚆)
  ◡  ◡

  ٩   ۶
(⚆ . ⚆)
  ◡  ◡

٩   ٩
(⚆ - ⚆)
 ◡  ◡

 ٩   ۶
(⚆ _ ⚆)
 ◡  ◡
`

// AnimationFrames manages animation frames for the mooncake character
type AnimationFrames struct {
	frames     [][]string // Each frame is a slice of 3 lines
	currentIdx int
	mu         sync.RWMutex
}

// LoadEmbeddedFrames loads animation frames from the embedded content
func LoadEmbeddedFrames() (*AnimationFrames, error) {
	return LoadFramesFromString(embeddedAnimationFrames)
}

// LoadFramesFromFile loads animation frames from a file
// Frames are expected to be 3 lines each, separated by blank lines
func LoadFramesFromFile(path string) (*AnimationFrames, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read animation file: %w", err)
	}

	return LoadFramesFromString(string(content))
}

// LoadFramesFromString loads animation frames from a string
// Frames are expected to be 3 lines each, separated by blank lines
func LoadFramesFromString(content string) (*AnimationFrames, error) {
	lines := strings.Split(content, "\n")
	var frames [][]string
	var currentFrame []string

	for _, line := range lines {
		// Blank line indicates end of current frame
		if strings.TrimSpace(line) == "" {
			if len(currentFrame) > 0 {
				// Only add frames that have exactly 3 lines
				if len(currentFrame) == 3 {
					frames = append(frames, currentFrame)
				}
				currentFrame = nil
			}
		} else {
			currentFrame = append(currentFrame, line)
		}
	}

	// Don't forget the last frame if file doesn't end with blank line
	if len(currentFrame) == 3 {
		frames = append(frames, currentFrame)
	}

	if len(frames) == 0 {
		return nil, fmt.Errorf("no valid animation frames found (expected 3 lines per frame)")
	}

	return &AnimationFrames{
		frames:     frames,
		currentIdx: 0,
	}, nil
}

// Next advances to the next frame and returns it
func (a *AnimationFrames) Next() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.currentIdx = (a.currentIdx + 1) % len(a.frames)
	return a.frames[a.currentIdx]
}

// Current returns the current frame without advancing
func (a *AnimationFrames) Current() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.frames[a.currentIdx]
}

// FrameCount returns the total number of frames
func (a *AnimationFrames) FrameCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return len(a.frames)
}
