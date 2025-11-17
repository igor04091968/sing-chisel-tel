#!/bin/bash

# Check if a commit message is provided
if [ -z "$1" ]; then
  echo "Error: Commit message is required."
  echo "Usage: $0 \"Your commit message\""
  exit 1
fi

# Navigate to the project root directory
cd "$(dirname "$0")"

echo "Staging all changes..."
git add .

echo "Committing changes..."
# Check if there are any changes to commit
if git diff --cached --exit-code; then
  echo "No changes to commit."
else
  git commit -m "$1"
fi

echo "Pushing changes to origin/main..."
git push origin main

echo "Git push script finished."