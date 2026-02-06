# ImageMagick - Image Processing Powerhouse

Comprehensive image manipulation toolkit for converting, editing, and processing images from the command line.

## Quick Start
```yaml
- preset: imagemagick
```

## Features
- **Format conversion**: Convert between 200+ image formats
- **Resizing & scaling**: Batch resize with quality preservation
- **Compositing**: Layer multiple images with blending modes
- **Effects**: Blur, sharpen, color correction, distortion
- **Batch processing**: Process thousands of images via scripts
- **Cross-platform**: Linux, macOS, Windows support

## Basic Usage
```bash
# Convert format
convert input.png output.jpg

# Resize image
convert input.jpg -resize 800x600 output.jpg

# Create thumbnail
convert input.jpg -thumbnail 200x200 thumb.jpg

# Add border
convert input.jpg -border 10x10 -bordercolor black output.jpg

# Rotate image
convert input.jpg -rotate 90 output.jpg

# Apply blur
convert input.jpg -blur 0x8 output.jpg

# Get image info
identify image.jpg
identify -verbose image.jpg

# Batch convert
mogrify -format png *.jpg

# Create GIF animation
convert -delay 100 frame*.png animation.gif
```

## Configuration
- **Policy**: `/etc/ImageMagick-7/policy.xml` (security policies)
- **Cache**: `~/.cache/ImageMagick/`
- **Temp files**: `/tmp/magick-*`

## Real-World Examples

### Batch Image Optimization for Web
```bash
# Optimize JPEGs for web (reduce quality, strip metadata)
mogrify -strip -interlace Plane -quality 85 *.jpg

# Resize all images to max width 1920px
mogrify -resize '1920x>' *.jpg

# Create thumbnails
for img in *.jpg; do
  convert "$img" -thumbnail 300x300^ -gravity center -extent 300x300 "thumb_$img"
done
```

### Create Social Media Assets
```bash
# Instagram post (1080x1080)
convert input.jpg -resize 1080x1080^ -gravity center -extent 1080x1080 instagram.jpg

# Twitter header (1500x500)
convert input.jpg -resize 1500x500^ -gravity center -extent 1500x500 twitter-header.jpg

# Add watermark
convert photo.jpg watermark.png -gravity southeast -geometry +10+10 -composite watermarked.jpg
```

### PDF Operations
```bash
# Convert PDF to images
convert -density 300 document.pdf page-%03d.png

# Create PDF from images
convert *.jpg output.pdf

# Extract specific page
convert document.pdf[2] page-3.png
```

### CI/CD Image Processing
```yaml
- name: Optimize product images
  shell: |
    mogrify -path optimized/ \
      -strip \
      -quality 85 \
      -resize '2000x>' \
      product_photos/*.jpg

- name: Generate thumbnails
  shell: |
    mkdir -p thumbnails
    mogrify -path thumbnails/ \
      -thumbnail 400x400^ \
      -gravity center \
      -extent 400x400 \
      product_photos/*.jpg
```

### Color Correction
```bash
# Auto-level contrast
convert input.jpg -auto-level output.jpg

# Normalize brightness
convert input.jpg -normalize output.jpg

# Adjust brightness/contrast
convert input.jpg -brightness-contrast 10x20 output.jpg

# Convert to grayscale
convert input.jpg -colorspace Gray output.jpg
```

## Agent Use
- Automate image optimization in build pipelines
- Generate responsive image variants for web applications
- Process user uploads (resize, compress, sanitize)
- Create social media graphics programmatically
- Batch convert and optimize product images
- Generate PDF documents from images

## Advanced Configuration
```yaml
- preset: imagemagick
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove ImageMagick |

## Troubleshooting

### Permission Errors with PDFs
```bash
# Check policy file
cat /etc/ImageMagick-7/policy.xml | grep PDF

# Enable PDF processing (edit policy.xml)
sudo sed -i 's/rights="none" pattern="PDF"/rights="read|write" pattern="PDF"/' /etc/ImageMagick-7/policy.xml
```

### Memory Issues
```bash
# Limit memory usage
convert -limit memory 2GB -limit map 4GB input.jpg output.jpg

# Process large files in tiles
convert huge.tif -define tiff:tile-geometry=256x256 tiled.tif
```

### Quality Loss
```bash
# Preserve quality when resizing
convert input.jpg -resize 50% -quality 95 output.jpg

# Use sampling for better quality
convert input.jpg -filter Lanczos -resize 800x600 output.jpg
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ✅ Windows (installer)

## Uninstall
```yaml
- preset: imagemagick
  with:
    state: absent
```

## Resources
- Official docs: https://imagemagick.org/
- Usage examples: https://imagemagick.org/Usage/
- GitHub: https://github.com/ImageMagick/ImageMagick
- Search: "imagemagick convert examples", "imagemagick batch processing", "imagemagick optimization"
