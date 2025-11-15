#!/bin/bash

# Navigate to the project root directory
cd "$(dirname "$0")"

echo "Staging all changes..."
git add .

echo "Committing changes..."
# Check if there are any changes to commit
if git diff --cached --exit-code; then
  echo "No changes to commit."
else
  git commit -m "Automated commit by Gemini CLI"
fi

echo "Pushing changes to origin/main..."
git push origin main

echo "Git push script finished."