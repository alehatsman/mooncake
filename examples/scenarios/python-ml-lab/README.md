# Python ML Lab Setup

Set up a complete Python machine learning environment on Ubuntu with popular ML libraries.

## What This Does

This scenario demonstrates:
- Installing Python 3 and pip
- Creating a Python virtual environment
- Installing ML packages (numpy, pandas, matplotlib, scikit-learn, jupyter)
- Setting up a workspace for ML projects
- Running a simple ML demo script

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed
- Internet connection for package downloads

## Files

- `setup.yml` - Main playbook
- `files/requirements.txt` - Python package requirements
- `files/hello_ml.py` - Sample ML demonstration script

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom workspace location
mooncake run setup.yml --var workspace_dir=$HOME/my-ml-workspace
```

## Variables

You can customize these variables:

- `workspace_dir` (default: `$HOME/ml-workspace`) - Workspace directory path
- `venv_dir` (default: `{{ workspace_dir }}/venv`) - Virtual environment path
- `python_version` (default: `3`) - Python version

## What Gets Installed

### System Packages
- python3
- python3-pip
- python3-venv

### Python ML Packages
- numpy - Numerical computing
- pandas - Data analysis
- matplotlib - Plotting and visualization
- scikit-learn - Machine learning algorithms
- jupyter - Interactive notebooks
- seaborn - Statistical data visualization
- scipy - Scientific computing

## Using Your ML Environment

### Activate Virtual Environment

```bash
source ~/ml-workspace/venv/bin/activate
```

### Run Sample Script

```bash
source ~/ml-workspace/venv/bin/activate
python3 ~/ml-workspace/hello_ml.py
```

### Start Jupyter Notebook

```bash
source ~/ml-workspace/venv/bin/activate
jupyter notebook --notebook-dir=~/ml-workspace/notebooks
```

Then open your browser to the URL shown (usually http://localhost:8888).

### Create Your First ML Project

```bash
cd ~/ml-workspace
source venv/bin/activate

# Create a new Python script
nano my_analysis.py

# Or create a new notebook
jupyter notebook notebooks/
```

## Sample Script

The included `hello_ml.py` demonstrates:
1. NumPy - Creating and manipulating arrays
2. Pandas - Creating and analyzing DataFrames
3. Scikit-learn - Training a simple classification model

## Cleanup

To remove the ML environment:

```bash
rm -rf ~/ml-workspace
sudo apt-get remove --purge python3-pip python3-venv
```

## Learning Points

This example teaches:
- Installing system packages with apt
- Creating Python virtual environments
- Installing Python packages with pip
- Managing workspace directories
- Running Python scripts from Mooncake
- Using assertions to verify installations
- Organizing ML project structure

## Next Steps

After setup, try:
- Creating Jupyter notebooks in `~/ml-workspace/notebooks/`
- Installing additional packages: `pip install tensorflow pytorch`
- Following scikit-learn tutorials
- Exploring kaggle datasets
