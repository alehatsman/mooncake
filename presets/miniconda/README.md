# Miniconda - Python Environment Manager

Miniconda is a lightweight Conda distribution that includes only essential package management tools. Unlike Anaconda, it ships minimal packages but provides full access to conda's 20,000+ curated libraries. Perfect for developers who want fast installation and control over their dependencies.

## Quick Start

```yaml
- preset: miniconda
```

After installation:

```bash
# Verify installation
conda --version

# Create your first environment
conda create -n myproject python=3.11
conda activate myproject
```

## Features

- **Lightweight**: Minimal footprint (~150MB vs Anaconda's 3GB)
- **Full package access**: 20,000+ packages via conda-forge and defaults channels
- **Environment isolation**: Create independent Python environments for different projects
- **Cross-platform**: Consistent behavior on Linux, macOS, Windows
- **Reproducible**: Export and share environment specifications with environment.yml
- **Binary packages**: Pre-compiled packages avoid C compiler dependency

## Basic Usage

```bash
# Check version and setup
conda --version
conda info

# Create and activate environment
conda create -n myenv python=3.11
conda activate myenv

# Install packages
conda install numpy pandas matplotlib

# Deactivate environment
conda deactivate

# List environments
conda env list
```

## Advanced Configuration

```yaml
- preset: miniconda
  with:
    install_path: ~/my-python-env
    init_shell: true
    state: present
```

**Key Parameters:**
- `install_path`: Custom installation location (default: ~/miniconda3)
- `init_shell`: Automatically configure bash/zsh for conda activation (default: true)
- `state`: Install or remove (present/absent)

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Miniconda |
| install_path | string | ~/miniconda3 | Installation directory path |
| init_shell | bool | true | Initialize conda in shell (bash/zsh) |
| create_base_env | bool | false | Use base environment (isolation not recommended) |

## Platform Support

- ✅ Linux (x86_64, ARM64)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (via WSL2 or native)
- ✅ Cloud environments (AWS, GCP, Azure)

## Configuration

- **Install directory**: `~/miniconda3` (or custom path)
- **Environments**: `~/miniconda3/envs/` (isolated Python environments)
- **Packages cache**: `~/miniconda3/pkgs/`
- **Config file**: `~/.condarc` (channel settings, defaults)
- **Shell init**: `~/.bashrc`, `~/.zshrc` (conda activation script)

## Real-World Examples

### Data Science Project Setup

```bash
# Create isolated environment
conda create -n datasci python=3.11

# Activate and install data tools
conda activate datasci
conda install numpy pandas scikit-learn matplotlib jupyter

# Start analysis
jupyter lab

# Save environment for sharing
conda env export > environment.yml
```

### Machine Learning Workflow

```yaml
# Environment specification (save as environment.yml)
name: ml-project
channels:
  - conda-forge
  - defaults
dependencies:
  - python=3.11
  - pytorch::pytorch
  - pytorch::torchvision
  - pytorch::pytorch-cuda=11.8
  - numpy
  - pandas
  - pip
  - pip:
    - transformers
    - datasets
```

Recreate environment:

```bash
conda env create -f environment.yml
conda activate ml-project
```

### Development Environment with Multiple Tools

```bash
# Create environment with build tools
conda create -n devenv python=3.11 \
  git cmake make gcc_linux-64 gxx_linux-64

conda activate devenv
pip install pytest black mypy  # Add pip packages
```

### Package Distribution

```bash
# Export exact packages for CI/CD
conda env export --file exact-spec.yml

# Share with team
git add environment.yml
git commit -m "Update Python dependencies"

# Teammate recreates exact environment
conda env create -f environment.yml
```

## Agent Use

- Provision isolated Python environments for different ML models and experiments
- Reproduce exact dependencies for CI/CD pipelines and automated testing
- Manage multiple Python versions for legacy/new code compatibility
- Build reproducible data pipelines with pinned package versions
- Deploy machine learning models with guaranteed dependencies
- Automate environment setup for development teams

## Troubleshooting

### Slow package resolution

Use libmamba solver for faster dependency resolution:

```bash
conda install -n base conda-libmamba-solver
conda config --set solver libmamba
```

### Conda command not found after installation

Manually initialize shell:

```bash
~/miniconda3/bin/conda init bash  # or zsh
source ~/.bashrc  # or ~/.zshrc
```

### Environment corruption

Repair with conda:

```bash
conda clean --all
conda update -n base -c defaults conda
```

### Disk space issues

Clean up unused packages:

```bash
conda clean --all --yes
conda clean --packages --yes
```

### Package not found in channels

Search multiple channels:

```bash
conda search package-name -c conda-forge
conda install -c conda-forge package-name
```

## Uninstall

```yaml
- preset: miniconda
  with:
    state: absent
```

**Important**: Before uninstalling, backup any custom environments or data stored in the Miniconda directory.

```bash
# Export important environments
conda env export -n myenv > myenv-backup.yml

# Then uninstall
```

## Resources

- Official docs: https://docs.conda.io/projects/miniconda/
- Conda cheat sheet: https://docs.conda.io/projects/conda/en/latest/user-guide/cheatsheet.html
- Conda-forge: https://conda-forge.org/
- GitHub: https://github.com/conda/miniconda
- Search: "conda environment management", "Python virtual environments conda", "reproducible Python environments"
