#!/usr/bin/env python3
"""
Simple Machine Learning Demo
Demonstrates basic usage of numpy, pandas, and scikit-learn
"""

import numpy as np
import pandas as pd
from sklearn.datasets import make_classification
from sklearn.model_selection import train_test_split
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import accuracy_score

def main():
    print("=" * 60)
    print("Welcome to Python ML Lab!")
    print("=" * 60)
    print()

    # NumPy demo
    print("1. NumPy Demo - Creating arrays")
    print("-" * 60)
    array = np.array([1, 2, 3, 4, 5])
    print(f"Array: {array}")
    print(f"Mean: {array.mean()}")
    print(f"Sum: {array.sum()}")
    print()

    # Pandas demo
    print("2. Pandas Demo - Creating a DataFrame")
    print("-" * 60)
    data = {
        'Name': ['Alice', 'Bob', 'Charlie', 'Diana'],
        'Age': [25, 30, 35, 28],
        'Score': [92, 88, 95, 89]
    }
    df = pd.DataFrame(data)
    print(df)
    print(f"\nAverage Score: {df['Score'].mean():.2f}")
    print()

    # Simple ML demo
    print("3. Scikit-Learn Demo - Simple Classification")
    print("-" * 60)

    # Generate synthetic dataset
    X, y = make_classification(
        n_samples=100,
        n_features=4,
        n_informative=3,
        n_redundant=1,
        random_state=42
    )

    # Split dataset
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42
    )

    # Train model
    model = LogisticRegression(random_state=42)
    model.fit(X_train, y_train)

    # Predict and evaluate
    y_pred = model.predict(X_test)
    accuracy = accuracy_score(y_test, y_pred)

    print(f"Dataset size: {X.shape[0]} samples, {X.shape[1]} features")
    print(f"Training set: {X_train.shape[0]} samples")
    print(f"Test set: {X_test.shape[0]} samples")
    print(f"Model accuracy: {accuracy * 100:.2f}%")
    print()

    print("=" * 60)
    print("Demo complete! Your ML environment is ready.")
    print("=" * 60)

if __name__ == "__main__":
    main()
