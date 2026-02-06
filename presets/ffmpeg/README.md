# FFmpeg - Multimedia Processing Framework

Complete, cross-platform solution for recording, converting, and streaming audio and video. Industry-standard tool for multimedia processing.

## Quick Start
```yaml
- preset: ffmpeg
```

## Features
- **Universal codec support**: Read and write virtually all audio/video formats
- **Hardware acceleration**: GPU encoding/decoding (NVENC, VAAPI, VideoToolbox)
- **Streaming**: RTMP, HLS, DASH live streaming protocols
- **Professional quality**: Used in production by Netflix, YouTube, and media companies
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Convert video format
ffmpeg -i input.mov output.mp4

# Extract audio from video
ffmpeg -i video.mp4 -vn -acodec copy audio.aac

# Resize video
ffmpeg -i input.mp4 -vf scale=1280:720 output.mp4

# Create GIF from video
ffmpeg -i video.mp4 -vf "fps=10,scale=640:-1" output.gif

# Compress video
ffmpeg -i input.mp4 -crf 28 -preset fast output.mp4

# Get video information
ffprobe -v quiet -print_format json -show_format -show_streams video.mp4
```

## Advanced Configuration
```yaml
- preset: ffmpeg
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove FFmpeg |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported in this preset)

## Configuration
- **Binaries**: `ffmpeg`, `ffprobe`, `ffplay`
- **Default codecs**: H.264, H.265, VP8, VP9, AAC, MP3, Opus
- **Hardware acceleration**: Automatically detected and enabled when available

## Real-World Examples

### Video Processing Pipeline
```bash
# Convert and compress for web
ffmpeg -i raw-video.mov \
  -c:v libx264 -preset slow -crf 22 \
  -c:a aac -b:a 128k \
  -vf "scale=1920:1080" \
  output.mp4

# Create multiple resolutions for adaptive streaming
for res in 1080 720 480; do
  ffmpeg -i input.mp4 -vf "scale=-2:${res}" \
    -c:v libx264 -crf 23 -c:a copy output_${res}p.mp4
done
```

### Screen Recording Processing
```bash
# Remove silence from recording
ffmpeg -i screen-recording.mp4 \
  -af silenceremove=1:0:-50dB \
  -c:v copy output.mp4

# Add logo watermark
ffmpeg -i video.mp4 -i logo.png \
  -filter_complex "overlay=W-w-10:10" \
  watermarked.mp4
```

### Live Streaming
```bash
# Stream to RTMP server (YouTube, Twitch)
ffmpeg -re -i video.mp4 \
  -c:v libx264 -preset veryfast -maxrate 3000k -bufsize 6000k \
  -c:a aac -b:a 128k \
  -f flv rtmp://a.rtmp.youtube.com/live2/YOUR_STREAM_KEY

# Create HLS stream for web playback
ffmpeg -i input.mp4 \
  -codec: copy -start_number 0 \
  -hls_time 10 -hls_list_size 0 \
  -f hls output.m3u8
```

## Agent Use
- Transcode video files in batch processing pipelines
- Generate thumbnails from videos for media libraries
- Extract audio tracks for transcription services
- Convert media formats for cross-platform compatibility
- Create video previews and GIF animations
- Process user-uploaded media in web applications

## Troubleshooting

### Codec not found
```bash
# List available codecs
ffmpeg -codecs | grep -i h264

# Install codec packages (Ubuntu)
sudo apt-get install ubuntu-restricted-extras

# Use software codec instead of hardware
ffmpeg -i input.mp4 -c:v libx264 output.mp4
```

### Out of memory errors
```bash
# Limit buffer size for large files
ffmpeg -i huge-video.mov -max_muxing_queue_size 1024 output.mp4

# Use streaming mode
ffmpeg -re -i input.mp4 -c copy output.mp4
```

### Audio/video sync issues
```bash
# Fix sync offset (+500ms)
ffmpeg -i input.mp4 -itsoffset 0.5 -i input.mp4 -map 0:v -map 1:a -c copy output.mp4

# Re-encode with specific framerate
ffmpeg -i input.mp4 -r 30 -c:v libx264 -c:a copy output.mp4
```

## Uninstall
```yaml
- preset: ffmpeg
  with:
    state: absent
```

## Resources
- Official docs: https://ffmpeg.org/documentation.html
- GitHub: https://github.com/FFmpeg/FFmpeg
- Wiki: https://trac.ffmpeg.org/wiki
- Codec guide: https://trac.ffmpeg.org/wiki/Encode/H.264
- Search: "ffmpeg video conversion", "ffmpeg streaming", "ffmpeg compression guide"
