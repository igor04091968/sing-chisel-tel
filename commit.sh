#!/bin/bash
# Exit immediately if a command exits with a non-zero status.
set -e
dt=`date +%d:%m:%Y:%H:%M`
#source ~/.bashrc
PROJECT_DIR="/mnt/usb_hdd1/Projects/sing-chisel-tel"

echo "INFO: This script assumes TF_VAR_github_user and TF_VAR_github_token are set in your environment."
if [ -z "${TF_VAR_github_user}" ] || [ -z "${TF_VAR_github_token}" ]; then
  echo "ERROR: TF_VAR_github_user and TF_VAR_github_token must be set."
  echo "Example: export TF_VAR_github_user=\"your-name\""
  echo "         export TF_VAR_github_token=\"YOUR_PAT_HERE\""
  exit 1
fi

echo "Changing to project directory: $PROJECT_DIR"
cd "$PROJECT_DIR"

echo "--> Stopping and removing existing container (if any)..."
docker stop sing-chisel-tel-container || true
docker rm sing-chisel-tel-container || true

echo "--> Tainting the Docker image to force a rebuild..."
terraform taint docker_image.sing_chisel_tel_image || true

echo "--> Running terraform init..."
terraform init -reconfigure

echo "--> Running terraform plan..."
terraform plan -out=tfplan

echo "--> Running terraform apply..."
terraform apply -auto-approve tfplan

echo "--> Adding changes to git..."
#############################################################
git add .
echo "--> Committing changes..."
# We use || true in case there are no changes to commit
################
#############################################
git commit -m "Automated commit via commit.sh $dt" || true
echo "--> Pushing changes to GitHub..."
# The variables are expanded here, when the script is run
###########################################################
git push "https://${TF_VAR_github_user}:${TF_VAR_github_token}@github.com/igor04091968/sing-chisel-tel.git" main
echo "Automation script finished successfully!"
