# Jupyter - Interactive Computing Environment

Interactive notebooks for data science, visualization, and scientific computing with support for 40+ programming languages.

## Quick Start
```yaml
- preset: jupyter
```

## Features
- **JupyterLab**: Next-generation web-based interface
- **Multiple kernels**: Python, R, Julia, and 40+ languages
- **Rich output**: Interactive plots, LaTeX, HTML, widgets
- **Notebooks**: Combine code, visualizations, and narrative text
- **Extensions**: Extensive ecosystem of plugins
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Start JupyterLab (recommended)
jupyter lab
# Opens at http://localhost:8888

# Start classic Notebook interface
jupyter notebook

# Start on specific port
jupyter lab --port=8889

# Start without opening browser
jupyter lab --no-browser

# List running servers
jupyter server list

# Stop all servers
jupyter server stop
```

## Advanced Configuration
```yaml
- preset: jupyter
  with:
    install_lab: true            # Install JupyterLab (default: true)
    install_extensions: true     # Install common extensions
    configure_password: false    # Set notebook password
    enable_nbextensions: true    # Enable notebook extensions
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Jupyter |
| install_lab | bool | true | Install JupyterLab interface |
| install_extensions | bool | false | Install common extensions |
| configure_password | bool | false | Prompt for notebook password |
| enable_nbextensions | bool | true | Enable notebook extensions |

## Creating Notebooks
```bash
# Create new notebook in JupyterLab
# 1. Open JupyterLab (jupyter lab)
# 2. Click "+" to open launcher
# 3. Click "Python 3" under Notebooks

# Create from command line
touch my_analysis.ipynb

# Convert from Python script
jupytext --to notebook script.py
```

## Working with Notebooks
```python
# Install packages within notebook
!pip install pandas matplotlib seaborn

# Run shell commands
!ls -la
!echo "Current directory: $(pwd)"

# Magic commands
%matplotlib inline          # Display plots inline
%timeit sum(range(1000))   # Time code execution
%load script.py            # Load external script
%%time                     # Time entire cell execution

# Display rich output
from IPython.display import display, HTML, Image
display(HTML('<h1>Hello World</h1>'))
display(Image('plot.png'))
```

## Configuration
```bash
# Generate config file
jupyter lab --generate-config
# Creates ~/.jupyter/jupyter_lab_config.py

jupyter notebook --generate-config
# Creates ~/.jupyter/jupyter_notebook_config.py

# Set password
jupyter lab password
# Saves to ~/.jupyter/jupyter_server_config.json

# Example configuration (~/.jupyter/jupyter_lab_config.py)
c.ServerApp.port = 8888
c.ServerApp.open_browser = True
c.ServerApp.root_dir = '/path/to/notebooks'
c.ServerApp.token = ''  # Disable token authentication (use password instead)
```

## Kernel Management
```bash
# List installed kernels
jupyter kernelspec list

# Install Python kernel from virtual environment
source venv/bin/activate
python -m ipykernel install --user --name=myproject --display-name="Python (myproject)"

# Install R kernel
R
> install.packages('IRkernel')
> IRkernel::installspec(user = TRUE)

# Install Julia kernel
julia
> using Pkg
> Pkg.add("IJulia")

# Remove kernel
jupyter kernelspec uninstall myproject
```

## Extensions
```bash
# JupyterLab extensions (via pip)
pip install jupyterlab-git              # Git integration
pip install jupyterlab-lsp              # Language server protocol
pip install jupyterlab-vim              # Vim keybindings
pip install jupyter-dash                # Plotly Dash apps
pip install jupyterlab-code-formatter   # Code formatting

# List extensions
jupyter labextension list

# Build JupyterLab (after installing extensions)
jupyter lab build

# Classic notebook extensions
pip install jupyter_contrib_nbextensions
jupyter contrib nbextension install --user
jupyter nbextension enable codefolding/main
```

## Real-World Examples

### Data Analysis Workflow
```python
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

# Load data
df = pd.read_csv('data.csv')

# Explore
df.head()
df.describe()
df.info()

# Visualize
plt.figure(figsize=(10, 6))
sns.scatterplot(data=df, x='age', y='salary')
plt.title('Age vs Salary')
plt.show()

# Statistical analysis
from scipy import stats
correlation, p_value = stats.pearsonr(df['age'], df['salary'])
print(f"Correlation: {correlation:.3f}, p-value: {p_value:.3f}")
```

### Machine Learning Pipeline
```python
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import classification_report

# Split data
X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2)

# Train model
model = RandomForestClassifier(n_estimators=100)
model.fit(X_train, y_train)

# Evaluate
y_pred = model.predict(X_test)
print(classification_report(y_test, y_pred))

# Feature importance
import pandas as pd
feature_importance = pd.DataFrame({
    'feature': X.columns,
    'importance': model.feature_importances_
}).sort_values('importance', ascending=False)
```

### Interactive Widgets
```python
import ipywidgets as widgets
from IPython.display import display

# Slider widget
slider = widgets.IntSlider(
    value=50,
    min=0,
    max=100,
    step=1,
    description='Value:'
)

def on_value_change(change):
    print(f"New value: {change['new']}")

slider.observe(on_value_change, names='value')
display(slider)

# Interactive plot
@widgets.interact(freq=(0.1, 2.0, 0.1))
def plot_sine(freq=1.0):
    x = np.linspace(0, 10, 1000)
    y = np.sin(2 * np.pi * freq * x)
    plt.plot(x, y)
    plt.show()
```

## Exporting Notebooks
```bash
# Export to HTML
jupyter nbconvert --to html notebook.ipynb

# Export to PDF (requires LaTeX)
jupyter nbconvert --to pdf notebook.ipynb

# Export to Python script
jupyter nbconvert --to python notebook.ipynb

# Export to Markdown
jupyter nbconvert --to markdown notebook.ipynb

# Execute and export
jupyter nbconvert --execute --to html notebook.ipynb

# Remove output cells
jupyter nbconvert --clear-output --inplace notebook.ipynb
```

## Remote Access
```bash
# Start server for remote access
jupyter lab --no-browser --ip=0.0.0.0 --port=8888

# With password (secure)
jupyter lab password  # Set password first
jupyter lab --no-browser --ip=0.0.0.0

# SSH tunnel (recommended)
# On server: jupyter lab --no-browser --port=8888
# On client: ssh -L 8888:localhost:8888 user@server
# Access at http://localhost:8888
```

## Keyboard Shortcuts

**Command Mode:**
- `A` - Insert cell above
- `B` - Insert cell below
- `D D` - Delete cell
- `M` - Markdown cell
- `Y` - Code cell
- `C` - Copy cell
- `V` - Paste cell
- `Z` - Undo delete

**Edit Mode:**
- `Shift + Enter` - Run cell, select below
- `Ctrl + Enter` - Run cell
- `Alt + Enter` - Run cell, insert below
- `Tab` - Code completion
- `Shift + Tab` - Tooltip
- `Ctrl + ]` - Indent
- `Ctrl + [` - Dedent

## CI/CD Integration
```bash
# Execute notebook in CI pipeline
jupyter nbconvert --execute --to notebook --inplace analysis.ipynb

# Parameterize notebooks
pip install papermill
papermill input.ipynb output.ipynb -p alpha 0.6 -p ratio 0.1

# Test notebooks
pytest --nbval notebooks/*.ipynb

# Convert to Python for linting
jupyter nbconvert --to python notebook.ipynb
pylint notebook.py
```

## Docker Integration
```dockerfile
FROM python:3.11-slim

# Install Jupyter
RUN pip install jupyterlab

# Set working directory
WORKDIR /notebooks

# Expose port
EXPOSE 8888

# Start JupyterLab
CMD ["jupyter", "lab", "--ip=0.0.0.0", "--allow-root", "--no-browser"]
```

```bash
# Build and run
docker build -t jupyter-env .
docker run -p 8888:8888 -v $(pwd):/notebooks jupyter-env
```

## Configuration Files
- **JupyterLab config**: `~/.jupyter/jupyter_lab_config.py`
- **Notebook config**: `~/.jupyter/jupyter_notebook_config.py`
- **Server config**: `~/.jupyter/jupyter_server_config.json`
- **Notebooks directory**: Current working directory (or set in config)
- **Extensions**: `~/.jupyter/lab/`
- **Default port**: 8888

## Performance Tips
```python
# Use cell magic for timing
%%time
# Code to time

# Profile code
%load_ext line_profiler
%lprun -f my_function my_function(args)

# Memory profiling
%load_ext memory_profiler
%memit large_array = np.random.rand(10000000)

# Display progress bars
from tqdm.notebook import tqdm
for i in tqdm(range(100)):
    # Long-running operation
    pass
```

## Agent Use
- Automated data analysis pipelines
- Machine learning model experimentation
- Scientific computing workflows
- Interactive dashboard generation
- Educational content creation
- Reproducible research documentation

## Troubleshooting

### Port already in use
```bash
jupyter lab --port=8889
```

### Kernel won't start
```bash
jupyter kernelspec list
jupyter kernelspec remove python3
python -m ipykernel install --user
```

### Extensions not loading
```bash
jupyter lab build --dev-build=False --minimize=True
jupyter lab clean
jupyter lab build
```

### Reset configuration
```bash
rm -rf ~/.jupyter/
jupyter lab --generate-config
```

## Uninstall
```yaml
- preset: jupyter
  with:
    state: absent
```

**Note:** Notebooks and configuration files are preserved. Remove manually if needed:
```bash
rm -rf ~/.jupyter/
```

## Resources
- Official docs: https://jupyter.org/documentation
- JupyterLab: https://jupyterlab.readthedocs.io/
- Notebook docs: https://jupyter-notebook.readthedocs.io/
- Gallery: https://github.com/jupyter/jupyter/wiki/A-gallery-of-interesting-Jupyter-Notebooks
- Search: "jupyter tutorial", "jupyterlab extensions", "jupyter notebook examples"

## Platform Support
- ✅ Linux (pip, conda)
- ✅ macOS (pip, conda, Homebrew)
- ❌ Windows (not yet supported)
