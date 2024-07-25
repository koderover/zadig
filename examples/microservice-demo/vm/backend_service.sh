#!/bin/bash
set -e

SERVICE_NAME="backend"
BACKEND_PORT=20219
BACKEND_EXEC="/root/microservice/backend/backend"
DEPLOY_DST="/root/microservice/backend/"
LOGFILE="/var/log/backend_service.log"

start_service() {
    echo "Starting $SERVICE_NAME service..."
    nohup $BACKEND_EXEC > /dev/null 2> /dev/null &
    echo "$SERVICE_NAME service started."
}

stop_service() {
    echo "Stopping $SERVICE_NAME service..."
    pid=$(lsof -i tcp:$BACKEND_PORT -t)
    if [ -n "$pid" ]; then
        kill -9 $pid
        echo "$SERVICE_NAME service stopped."
    else
        echo "$SERVICE_NAME service is not running."
    fi
}

restart_service() {
    echo "Restarting $SERVICE_NAME service..."
    stop_service
    start_service
    echo "$SERVICE_NAME service restarted."
}

status_service() {
    pid=$(lsof -i tcp:$BACKEND_PORT -t)
    if [ -n "$pid" ]; then
        echo "$SERVICE_NAME service is running with PID $pid."
    else
        echo "$SERVICE_NAME service is not running."
    fi
}

deploy_service() {
    echo "Starting Deploy $SERVICE_NAME service..."
    PACKAGE_NAME=$1
    DEPLOY_SRC="$DEPLOY_DST/$PACKAGE_NAME"
    echo "Deploying $SERVICE_NAME service with package $PACKAGE_NAME..."
    if [ -f "$DEPLOY_SRC" ]; then
        tar xvf "$DEPLOY_SRC" -C "$DEPLOY_DST"
        chmod +x "$DEPLOY_DST/$(basename $BACKEND_EXEC)"
        restart_service
        echo "Deployment of $PACKAGE_NAME completed."
    else
        echo "Deployment source $DEPLOY_SRC does not exist."
        exit 1
    fi

    rm -rf $DEPLOY_SRC
}

case "$1" in
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        restart_service
        ;;
    deploy)
        if [ -z "$2" ]; then
            echo "Usage: $0 deploy <package_name>"
            exit 1
        else
            deploy_service $2
        fi
        ;;  
    status)
        status_service
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac

exit 0