#!/bin/bash

# --- Конфигурация проекта ---
APP_NAME="sing-chisel-tel"
MAIN_GO_FILE="main.go" # Основной файл Go-приложения
IMAGE_NAME="${APP_NAME}-app:latest"
CONTAINER_NAME="${APP_NAME}-container"

# --- Цвета для вывода ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# --- Функции ---

# Функция для отображения сообщений
log_message() {
    local type=$1
    local message=$2
    case "$type" in
        "INFO") echo -e "${BLUE}[INFO]${NC} ${message}" ;;
        "SUCCESS") echo -e "${GREEN}[SUCCESS]${NC} ${message}" ;;
        "WARNING") echo -e "${YELLOW}[WARNING]${NC} ${message}" ;;
        "ERROR") echo -e "${RED}[ERROR]${NC} ${message}" ;;
        *) echo "${message}" ;;
    esac
}

# 1. Сборка Go-приложения
build_go_app() {
    log_message "INFO" "Запуск сборки Go-приложения..."
    CGO_ENABLED=0 go build -o "$APP_NAME" "$MAIN_GO_FILE"
    if [ $? -eq 0 ]; then
        log_message "SUCCESS" "Go-приложение успешно собрано: $APP_NAME"
    else
        log_message "ERROR" "Ошибка при сборке Go-приложения."
        exit 1
    fi
}

# 2. Тестирование Go-приложения
test_go_app() {
    log_message "INFO" "Запуск Go-тестов..."
    go test ./...
    if [ $? -eq 0 ]; then
        log_message "SUCCESS" "Все тесты Go успешно пройдены."
    else
        log_message "ERROR" "Go-тесты провалены."
        exit 1
    fi
}

# 3. Сборка Docker-образа
build_docker_image() {
    log_message "INFO" "Запуск сборки Docker-образа: $IMAGE_NAME"
    docker build -t "$IMAGE_NAME" .
    if [ $? -eq 0 ]; then
        log_message "SUCCESS" "Docker-образ успешно собран: $IMAGE_NAME"
    else
        log_message "ERROR" "Ошибка при сборке Docker-образа."
        exit 1
    fi
}

# 4. Развертывание (запуск) Docker-контейнера
deploy_docker_container() {
    log_message "INFO" "Развертывание Docker-контейнера: $CONTAINER_NAME"
    # Остановка и удаление существующего контейнера, если он запущен
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        log_message "WARNING" "Контейнер ${CONTAINER_NAME} уже существует. Остановка и удаление..."
        docker stop "$CONTAINER_NAME" > /dev/null
        docker rm "$CONTAINER_NAME" > /dev/null
    fi
    docker run -d --name "$CONTAINER_NAME" "$IMAGE_NAME"
    if [ $? -eq 0 ]; then
        log_message "SUCCESS" "Docker-контейнер ${CONTAINER_NAME} успешно запущен."
    else
        log_message "ERROR" "Ошибка при запуске Docker-контейнера."
        exit 1
    fi
}

# 5. Просмотр логов контейнера
view_container_logs() {
    log_message "INFO" "Просмотр логов контейнера: $CONTAINER_NAME"
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        docker logs "$CONTAINER_NAME"
    else
        log_message "WARNING" "Контейнер ${CONTAINER_NAME} не найден."
    fi
}

# 6. Остановка Docker-контейнера
stop_container() {
    log_message "INFO" "Остановка Docker-контейнера: $CONTAINER_NAME"
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        docker stop "$CONTAINER_NAME"
        if [ $? -eq 0 ]; then
            log_message "SUCCESS" "Контейнер ${CONTAINER_NAME} успешно остановлен."
        else
            log_message "ERROR" "Ошибка при остановке контейнера ${CONTAINER_NAME}."
        fi
    else
        log_message "WARNING" "Контейнер ${CONTAINER_NAME} не запущен или не существует."
    fi
}

# 7. Удаление Docker-контейнера
remove_container() {
    log_message "INFO" "Удаление Docker-контейнера: $CONTAINER_NAME"
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        docker stop "$CONTAINER_NAME" > /dev/null
    fi
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        docker rm "$CONTAINER_NAME"
        if [ $? -eq 0 ]; then
            log_message "SUCCESS" "Контейнер ${CONTAINER_NAME} успешно удален."
        else
            log_message "ERROR" "Ошибка при удалении контейнера ${CONTAINER_NAME}."
        fi
    else
        log_message "WARNING" "Контейнер ${CONTAINER_NAME} не существует."
    fi
}

# 8. Резервное копирование основного Go-файла
backup_main_go_file() {
    log_message "INFO" "Создание резервной копии ${MAIN_GO_FILE}..."
    if [ -f "$MAIN_GO_FILE" ]; then
        cp "$MAIN_GO_FILE" "${MAIN_GO_FILE}.bak"
        log_message "SUCCESS" "Резервная копия ${MAIN_GO_FILE}.bak создана."
    else
        log_message "WARNING" "Файл ${MAIN_GO_FILE} не найден. Резервная копия не создана."
    fi
}

# 9. Очистка Docker-системы
cleanup_docker_system() {
    log_message "WARNING" "Запуск полной очистки Docker-системы (удалит неиспользуемые образы, контейнеры, сети, кэш сборки)!"
    log_message "WARNING" "ВНИМАНИЕ: Это может повлиять на другие ваши Docker-проекты."
    read -p "Вы уверены, что хотите продолжить? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker system prune -a -f
        if [ $? -eq 0 ]; then
            log_message "SUCCESS" "Docker-система успешно очищена."
        else
            log_message "ERROR" "Ошибка при очистке Docker-системы."
        fi
    else
        log_message "INFO" "Очистка Docker-системы отменена."
    fi
}

# --- Основное меню ---
show_menu() {
    echo -e "${BLUE}--- Меню управления проектом ${APP_NAME} ---${NC}"
    echo "1. Собрать Go-приложение (go build)"
    echo "2. Запустить Go-тесты (go test)"
    echo "3. Собрать Docker-образ"
    echo "4. Развернуть (запустить) Docker-контейнер"
    echo "5. Просмотреть логи контейнера"
    echo "6. Остановить Docker-контейнер"
    echo "7. Удалить Docker-контейнер"
    echo "8. Создать резервную копию main.go"
    echo "9. Очистить Docker-систему (docker system prune -a -f)"
    echo "0. Выход"
    echo -e "${BLUE}---------------------------------------${NC}"
}

main() {
    while true; do
        show_menu
        read -p "Выберите действие (0-9): " choice
        case $choice in
            1) build_go_app ;;
            2) test_go_app ;;
            3) build_docker_image ;;
            4) deploy_docker_container ;;
            5) view_container_logs ;;
            6) stop_container ;;
            7) remove_container ;;
            8) backup_main_go_file ;;
            9) cleanup_docker_system ;;
            0) log_message "INFO" "Выход из скрипта."; exit 0 ;;
            *) log_message "WARNING" "Неверный выбор. Пожалуйста, введите число от 0 до 9." ;;
        esac
        echo # Добавить пустую строку для лучшей читаемости
        read -p "Нажмите Enter для продолжения..."
    done
}

# Запуск основного меню
main
