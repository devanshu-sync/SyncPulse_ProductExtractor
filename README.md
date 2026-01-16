# Product Extraction & Fuzzy Matching API

A high-performance Go (Golang) API designed to extract and map structured product information (Product Name, Brand, Category) from raw input text.

It uses a hybrid approach combining **Token Elimination** and **Levenshtein Distance (Fuzzy Matching)** to find the most accurate match from a loaded CSV dictionary.

## 🚀 Features

* **Fast In-Memory Lookup:** Loads the dictionary into memory for millisecond-level response times.
* **Hybrid Matching Logic:**
    1.  **Token Filtering:** Rapidly eliminates candidates that don't share tokens with the input.
    2.  **Fuzzy Scoring:** Calculates Levenshtein distance scores on remaining candidates to find the best match.
* **Normalization:** automatically handles HTML entities, special characters, and technical units (GB, TB, Hz, etc.).
* **REST API:** Simple JSON-based interface.

## 📋 Prerequisites

* [Go](https://go.dev/dl/) (version 1.18 or higher recommended)

## 🛠️ Setup & Installation

1.  **Clone the repository**
    ```bash
    git clone (https://github.com/devanshu-sync/SyncPulse_ProductExtractor.git)
    ```

2.  **Initialize the module**
    ```bash
    go mod init product-extractor
    go mod tidy
    ```

3.  **Prepare your Dictionary Data**
    Create a CSV file (e.g., `dictionary.csv`). The file **must** contain a header row with the following columns (case-insensitive):
    * `Product` (Required)
    * `Brand`
    * `Category`

    **Example `dictionary.csv`:**
    ```csv
    Product,Brand,Category
    iPhone 13 Pro 128GB,Apple,Smartphones
    Galaxy S21 Ultra,Samsung,Smartphones
    MacBook Air M1,Apple,Laptops
    ```

## 🏃‍♂️ Running the Server

You can run the server using default settings or customize it via Environment Variables.

### Standard Run
```bash
# Sets the path to your CSV and starts the server
export DICT_PATH="./dictionary.csv"
go run main.go
