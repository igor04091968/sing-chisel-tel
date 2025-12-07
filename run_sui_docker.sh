#!/bin/bash

IMAGE_NAME="s-ui-app"
CONTAINER_NAME="s-ui-container"
HOST_PORT="2097"
CONTAINER_PORT="2095"
DB_VOLUME_PATH="$(pwd)/db"

echo "--- Building Docker image: $IMAGE_NAME ---"
docker build -t "$IMAGE_NAME" .

if [ $? -ne 0 ]; then
    echo "Docker image build failed. Exiting."
    exit 1
fi

echo "--- Checking for existing container: $CONTAINER_NAME ---"
if docker ps -a --format '{{.Names}}' | grep -q "$CONTAINER_NAME"; then
    echo "Existing container '$CONTAINER_NAME' found. Removing it..."
    docker rm -f "$CONTAINER_NAME"
    if [ $? -ne 0 ]; then
        echo "Failed to remove existing container. Exiting."
        exit 1
    fi
fi

echo "--- Creating database volume directory if it doesn't exist ---"
mkdir -p "$DB_VOLUME_PATH"

echo "--- Running Docker container: $CONTAINER_NAME ---"
docker run -d \
    --cap-add=NET_RAW \
    -p "$HOST_PORT":"$CONTAINER_PORT" \
    -v "$DB_VOLUME_PATH":/app/db \
    --name "$CONTAINER_NAME" \
    "$IMAGE_NAME"

if [ $? -ne 0 ]; then
    echo "Docker container failed to start. Exiting."
    exit 1
fi

echo "--- Container '$CONTAINER_NAME' started successfully! ---"
echo "You can access the s-ui web interface at: http://localhost:$HOST_PORT"
echo "To view container logs: docker logs $CONTAINER_NAME"
echo "To stop the container: docker stop $CONTAINER_NAME"
echo "To remove the container: docker rm $CONTAINER_NAME"
