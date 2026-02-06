# Pandoc - Universal Document Converter

Convert documents between markup formats. Supports Markdown, HTML, LaTeX, Word, PDF, and 40+ other formats with powerful templating.

## Quick Start
```yaml
- preset: pandoc
```

## Features
- **Multi-format conversion**: 40+ input and output formats
- **Markdown extensions**: Tables, footnotes, math, syntax highlighting
- **Template system**: Customizable output with variables
- **Citation support**: BibTeX, CSL, bibliography management
- **PDF generation**: Via LaTeX, wkhtmltopdf, or Prince
- **Filters**: Lua and JSON filters for document transformation
- **Cross-platform**: Linux, macOS, Windows

## Basic Usage
```bash
# Convert Markdown to HTML
pandoc input.md -o output.html

# Convert Markdown to PDF
pandoc input.md -o output.pdf

# Convert Word to Markdown
pandoc input.docx -o output.md

# Convert HTML to Markdown
pandoc input.html -o output.md

# Convert LaTeX to Word
pandoc input.tex -o output.docx

# Convert Markdown to slides (reveal.js)
pandoc slides.md -t revealjs -o slides.html
```

## Advanced Configuration
```yaml
# Install Pandoc (default)
- preset: pandoc

# Uninstall Pandoc
- preset: pandoc
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support
- ✅ Linux (apt, dnf, pacman, tar.gz)
- ✅ macOS (Homebrew, pkg installer)
- ✅ Windows (msi installer, chocolatey)

## Configuration
- **Data directory**: `~/.local/share/pandoc/` (Linux), `~/Library/Application Support/pandoc/` (macOS)
- **Templates**: `~/.local/share/pandoc/templates/`
- **Filters**: `~/.local/share/pandoc/filters/`
- **No config file**: All options via command-line or defaults

## Document Conversion
```bash
# Set metadata
pandoc input.md -o output.html \
  --metadata title="My Document" \
  --metadata author="John Doe" \
  --metadata date="2024-01-01"

# Include table of contents
pandoc input.md -o output.html --toc

# Set TOC depth
pandoc input.md -o output.html --toc --toc-depth=2

# Number sections
pandoc input.md -o output.html --number-sections

# Standalone document (with headers)
pandoc input.md -o output.html --standalone

# Use custom template
pandoc input.md -o output.html --template=mytemplate.html
```

## PDF Generation
```bash
# Generate PDF (requires LaTeX)
pandoc input.md -o output.pdf

# Set PDF engine
pandoc input.md -o output.pdf --pdf-engine=xelatex

# Set paper size
pandoc input.md -o output.pdf -V papersize=a4

# Set margins
pandoc input.md -o output.pdf \
  -V geometry:margin=1in

# Set font
pandoc input.md -o output.pdf \
  -V mainfont="Times New Roman" \
  -V fontsize=12pt
```

## Markdown Extensions
```bash
# Enable specific extensions
pandoc input.md -o output.html \
  -f markdown+pipe_tables+footnotes+smart

# Disable extensions
pandoc input.md -o output.html \
  -f markdown-smart

# Common extensions:
# - pipe_tables: |---|---|
# - footnotes: [^1]
# - smart: Smart quotes and dashes
# - tex_math_dollars: $math$
# - fenced_code_attributes: ```python
# - definition_lists: Term : Definition
# - yaml_metadata_block: ---...---
```

## Citations and Bibliography
```bash
# Add bibliography
pandoc input.md -o output.html --bibliography=refs.bib

# Set citation style (CSL)
pandoc input.md -o output.html \
  --bibliography=refs.bib \
  --csl=chicago.csl

# Citation formats in Markdown:
# [@smith2020] - (Smith 2020)
# @smith2020 - Smith (2020)
# [@smith2020; @jones2021] - (Smith 2020; Jones 2021)
```

## Filters and Transformations
```bash
# Apply Lua filter
pandoc input.md -o output.html --lua-filter=transform.lua

# Apply JSON filter
pandoc input.md -o output.html --filter=./filter.py

# Chain multiple filters
pandoc input.md -o output.html \
  --lua-filter=filter1.lua \
  --lua-filter=filter2.lua \
  --filter=./postprocess.py
```

## Templates
```bash
# List default templates
pandoc -D html5 > default.html
pandoc -D latex > default.tex

# Use custom template
pandoc input.md -o output.html --template=custom.html

# Template variables:
# $title$ $author$ $date$
# $body$ $toc$
# $header-includes$
```

## Slide Generation
```bash
# Create reveal.js slides
pandoc slides.md -t revealjs -o slides.html \
  -V theme=moon \
  -V transition=fade

# Create Beamer slides (PDF)
pandoc slides.md -t beamer -o slides.pdf

# Create PowerPoint
pandoc slides.md -o slides.pptx

# Slide separators in Markdown:
# # Section
# ## Slide title
# Content...
#
# ## Next slide
```

## Real-World Examples

### Documentation Pipeline
```yaml
- name: Install Pandoc
  preset: pandoc

- name: Convert documentation to HTML
  shell: |
    for file in docs/*.md; do
      pandoc "$file" -o "html/$(basename $file .md).html" \
        --standalone \
        --toc \
        --template=docs-template.html \
        --css=style.css
    done
```

### Academic Paper
```bash
# Convert Markdown paper to PDF
pandoc paper.md -o paper.pdf \
  --bibliography=references.bib \
  --csl=apa.csl \
  --number-sections \
  --toc \
  -V geometry:margin=1in \
  -V fontsize=12pt \
  -V linestretch=2
```

### Multi-Format Publishing
```bash
# Generate multiple output formats
for format in html pdf docx epub; do
  pandoc book.md -o "book.$format" \
    --toc \
    --number-sections \
    --metadata title="My Book"
done
```

### README Conversion
```bash
# Convert GitHub Markdown to HTML
pandoc README.md -o README.html \
  -f gfm \
  --standalone \
  --css=github.css
```

## Advanced Features
```bash
# Include files
pandoc main.md chapter1.md chapter2.md -o book.pdf

# Set variables
pandoc input.md -o output.html \
  -V lang=en \
  -V documentclass=article

# Highlight style
pandoc input.md -o output.html --highlight-style=tango

# Self-contained HTML (embed images)
pandoc input.md -o output.html --self-contained

# Extract media
pandoc input.docx -o output.md --extract-media=./media
```

## Configuration Files
```yaml
# defaults.yaml
from: markdown
to: html
standalone: true
toc: true
number-sections: true
template: mytemplate.html
css: style.css

# Use with:
pandoc input.md -o output.html --defaults=defaults.yaml
```

## Agent Use
- Documentation generation
- Report automation
- Academic paper workflows
- Multi-format publishing
- Markdown to PDF conversion
- Static site generation
- API documentation rendering

## Troubleshooting

### PDF generation fails
Install LaTeX distribution:
```bash
# Linux
sudo apt-get install texlive-latex-base texlive-latex-extra

# macOS
brew install --cask mactex
```

### Missing templates
Create template directory:
```bash
mkdir -p ~/.local/share/pandoc/templates
pandoc -D html5 > ~/.local/share/pandoc/templates/default.html
```

### Citation errors
Install pandoc-citeproc:
```bash
# Built-in since Pandoc 2.11
# For older versions:
# apt-get install pandoc-citeproc  # Linux
# brew install pandoc-citeproc     # macOS
```

### Unicode errors
Specify Unicode engine for PDF:
```bash
pandoc input.md -o output.pdf --pdf-engine=xelatex
```

## Uninstall
```yaml
- preset: pandoc
  with:
    state: absent
```

## Resources
- Official docs: https://pandoc.org/MANUAL.html
- GitHub: https://github.com/jgm/pandoc
- Templates: https://github.com/jgm/pandoc-templates
- Filters: https://github.com/pandoc/lua-filters
- Search: "pandoc tutorial", "pandoc examples", "pandoc markdown to pdf"
