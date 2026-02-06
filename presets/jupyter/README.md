# Jupyter Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Start Jupyter Notebook
jupyter notebook

# Start JupyterLab (recommended)
jupyter lab

# Start on specific port
jupyter lab --port=8889

# List running servers
jupyter server list
```

## Configuration

- **Config directory:** `~/.jupyter/`
- **Notebooks directory:** Current working directory (default)
- **Default port:** 8888
- **Browser:** Opens automatically

## Common Operations

```bash
# Generate config file
jupyter notebook --generate-config

# Set password
jupyter notebook password

# Install kernel
python -m ipykernel install --user --name=myenv

# List kernels
jupyter kernelspec list

# Remove kernel
jupyter kernelspec uninstall myenv

# Export notebook
jupyter nbconvert --to html notebook.ipynb
jupyter nbconvert --to pdf notebook.ipynb
jupyter nbconvert --to python notebook.ipynb

# Run notebook non-interactively
jupyter nbconvert --execute --to notebook notebook.ipynb

# Clear output
jupyter nbconvert --clear-output --inplace notebook.ipynb
```

## JupyterLab Extensions

```bash
# Install extension
pip install jupyterlab-git

# List extensions
jupyter labextension list

# Build JupyterLab
jupyter lab build
```

## Remote Access

```bash
# Start without browser
jupyter lab --no-browser --port=8888

# Allow remote connections (be careful!)
jupyter lab --ip=0.0.0.0 --no-browser
```

## Keyboard Shortcuts

**Command Mode:**
- `A` - Insert cell above
- `B` - Insert cell below
- `D D` - Delete cell
- `M` - Change to Markdown
- `Y` - Change to Code

**Edit Mode:**
- `Shift + Enter` - Run cell
- `Ctrl + Enter` - Run cell (stay in cell)
- `Tab` - Code completion

## Python in Notebooks

```python
# Install package in current notebook
!pip install pandas

# Run shell command
!ls -la

# Display plots
%matplotlib inline
import matplotlib.pyplot as plt

# Time code execution
%timeit sum(range(100))

# Load external script
%load script.py
```

## Uninstall

```yaml
- preset: jupyter
  with:
    state: absent
```

**Note:** Notebooks and config files preserved after uninstall.
