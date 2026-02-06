# Miniconda Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Check version
conda --version

# Initialize shell (if not auto-done)
conda init bash  # or zsh

# Create environment
conda create -n myenv python=3.11

# Activate environment
conda activate myenv

# Deactivate
conda deactivate
```

## Configuration

- **Install directory:** `~/miniconda3` (default)
- **Environments:** `~/miniconda3/envs/`
- **Packages:** `~/miniconda3/pkgs/`
- **Config file:** `~/.condarc`

## Environment Management

```bash
# Create environment with packages
conda create -n myenv python=3.11 numpy pandas

# Create from file
conda env create -f environment.yml

# List environments
conda env list

# Remove environment
conda env remove -n myenv

# Export environment
conda env export > environment.yml

# Clone environment
conda create --name newenv --clone myenv
```

## Package Management

```bash
# Install package
conda install numpy
conda install -c conda-forge package-name

# Install specific version
conda install numpy=1.24.0

# Update package
conda update numpy

# Update all packages
conda update --all

# Remove package
conda remove numpy

# List installed packages
conda list

# Search for package
conda search scipy
```

## Channels

```bash
# Add channel
conda config --add channels conda-forge

# Set channel priority
conda config --set channel_priority strict

# List channels
conda config --show channels
```

## Common Workflows

```bash
# Data science environment
conda create -n datascience python=3.11 \
  numpy pandas matplotlib scikit-learn jupyter

# Machine learning environment
conda create -n ml python=3.11 \
  pytorch torchvision tensorflow keras

# Activate and work
conda activate datascience
jupyter lab
```

## Conda vs Pip

```bash
# Use conda first, pip second
conda install package-name
# If not available in conda:
pip install package-name

# In environment.yml
channels:
  - conda-forge
  - defaults
dependencies:
  - python=3.11
  - numpy
  - pip:
    - some-pip-only-package
```

## Clean Up

```bash
# Remove unused packages
conda clean --all

# Remove cached packages
conda clean --packages

# Remove tarballs
conda clean --tarballs
```

## Uninstall

```yaml
- preset: miniconda
  with:
    state: absent
```

**Note:** Removes miniconda installation. Backup environments first!
