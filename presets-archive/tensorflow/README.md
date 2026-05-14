# TensorFlow Preset

**Status:** ✓ Installed successfully

## Quick Start

```python
import tensorflow as tf

# Check version
print(tf.__version__)

# Check GPU availability
print(f"GPU available: {tf.config.list_physical_devices('GPU')}")
print(f"Built with CUDA: {tf.test.is_built_with_cuda()}")

# Create tensor
x = tf.constant([1, 2, 3])
print(x)
```

## Configuration

- **Installation:** Via pip
- **GPU support:** Auto-detected if available (CUDA/cuDNN required)
- **Device:** CPU by default, GPU if available

## Basic Operations

```python
import tensorflow as tf

# Create tensors
x = tf.zeros([3, 4])
y = tf.ones([3, 4])
z = tf.random.normal([3, 4])

# Operations
result = tf.add(x, y)
result = tf.matmul(x, tf.transpose(y))

# Convert to numpy
np_array = x.numpy()

# From numpy
x = tf.convert_to_tensor(np_array)

# Check device placement
print(x.device)
```

## Keras API (Recommended)

```python
from tensorflow import keras
from tensorflow.keras import layers

# Sequential model
model = keras.Sequential([
    layers.Dense(128, activation='relu', input_shape=(784,)),
    layers.Dropout(0.2),
    layers.Dense(10, activation='softmax')
])

# Compile
model.compile(
    optimizer='adam',
    loss='sparse_categorical_crossentropy',
    metrics=['accuracy']
)

# Train
history = model.fit(
    x_train, y_train,
    epochs=10,
    validation_data=(x_val, y_val),
    batch_size=32
)

# Evaluate
test_loss, test_acc = model.evaluate(x_test, y_test)

# Predict
predictions = model.predict(x_new)
```

## Functional API

```python
from tensorflow.keras import Input, Model
from tensorflow.keras.layers import Dense, Concatenate

# Inputs
input1 = Input(shape=(64,))
input2 = Input(shape=(32,))

# Layers
x1 = Dense(64, activation='relu')(input1)
x2 = Dense(64, activation='relu')(input2)

# Concatenate
merged = Concatenate()([x1, x2])
output = Dense(10, activation='softmax')(merged)

# Model
model = Model(inputs=[input1, input2], outputs=output)
```

## Data Pipeline

```python
# From numpy
dataset = tf.data.Dataset.from_tensor_slices((x_train, y_train))

# Shuffle and batch
dataset = dataset.shuffle(1000).batch(32)

# Map function
def preprocess(x, y):
    x = tf.cast(x, tf.float32) / 255.0
    return x, y

dataset = dataset.map(preprocess)

# Prefetch
dataset = dataset.prefetch(tf.data.AUTOTUNE)

# Use in training
model.fit(dataset, epochs=10)
```

## Save/Load Model

```python
# Save entire model
model.save('my_model.h5')
model.save('my_model')  # SavedModel format

# Load model
model = keras.models.load_model('my_model.h5')

# Save weights only
model.save_weights('weights.h5')

# Load weights
model.load_weights('weights.h5')
```

## Custom Training Loop

```python
@tf.function
def train_step(x, y):
    with tf.GradientTape() as tape:
        predictions = model(x, training=True)
        loss = loss_fn(y, predictions)

    gradients = tape.gradient(loss, model.trainable_variables)
    optimizer.apply_gradients(zip(gradients, model.trainable_variables))

    return loss

# Training loop
for epoch in range(epochs):
    for x_batch, y_batch in train_dataset:
        loss = train_step(x_batch, y_batch)
```

## TensorBoard

```python
# Callback
tensorboard_callback = keras.callbacks.TensorBoard(
    log_dir='./logs',
    histogram_freq=1
)

# Train with callback
model.fit(
    x_train, y_train,
    epochs=10,
    callbacks=[tensorboard_callback]
)

# View in browser
# tensorboard --logdir=./logs
```

## Common Layers

```python
# Dense
layers.Dense(units, activation='relu')

# Convolution
layers.Conv2D(filters, kernel_size, activation='relu')
layers.MaxPooling2D(pool_size=(2, 2))

# Recurrent
layers.LSTM(units, return_sequences=True)
layers.GRU(units)

# Normalization
layers.BatchNormalization()
layers.LayerNormalization()

# Regularization
layers.Dropout(0.5)

# Attention
layers.MultiHeadAttention(num_heads, key_dim)
```

## GPU Memory Management

```python
# Allow memory growth
gpus = tf.config.list_physical_devices('GPU')
if gpus:
    for gpu in gpus:
        tf.config.experimental.set_memory_growth(gpu, True)

# Set memory limit
tf.config.set_logical_device_configuration(
    gpus[0],
    [tf.config.LogicalDeviceConfiguration(memory_limit=4096)]
)
```

## Uninstall

```yaml
- preset: tensorflow
  with:
    state: absent
```

**Note:** Model checkpoints and logs preserved after uninstall.
