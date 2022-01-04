#!/bin/bash

#
# Startup script for Bean
# 
# chkconfig: - 80 20
# description: Bean, a bare metal API development engine.
# processname: bean
#

PATH=/usr/local/bin:$PATH
export PATH

ARGS="$@"
ROOT_DIR=`echo $(cd $(dirname $0);pwd)`
MASTER_PID_FILE=/var/run/bean/bean.pid
KILL_OPTION=-SIGTERM
DAEMON=bin/{{ .PkgName }}
TIME_OUT=60

black='\e[0;30m'
red='\e[0;31m'
green='\e[0;32m'
yellow='\e[0;33m'
blue='\e[0;34m'
magenta='\e[0;35m'
darkCyan='\e[0;36m'
white='\e[0;37m'
NC='\e[0;m'

if [  -n "$(uname -a | grep Ubuntu)" ]; then
    if [ -a /lib/lsb/init-functions ]
    then
        . /lib/lsb/init-functions
    fi
else
    if [ -a /etc/rc.d/init.d/functions ]
    then
        . /etc/rc.d/init.d/functions
    fi
fi

if [ ! -f "$ROOT_DIR/$DAEMON" ]; then
    ROOT_DIR=/var/www/bean
fi

if [ ! -f "$ROOT_DIR/$DAEMON" ]; then
    printf "$DAEMON: ${red}Not found.${NC}\n"
    
    exit 0    
fi

function isProcessRunning()
{
    ps -f $1 &>/dev/null
}

function spinner()
{
    chars="/-\|"

    while :; do
        for (( i=0; i<${#chars}; i++ )); do
            sleep 0.5
            echo -en "$1 ${chars:$i:1}" "\r"
        done
    done
}

# START 
function start()
{
    sudo mkdir -p /var/run/bean
    sudo chown $(whoami).$(whoami) /var/run/bean
    
    ARGS=`echo "$ARGS" | sed 's/start//g'`
    
    if [ -a $MASTER_PID_FILE ]
    then
        printf "$DAEMON: ${yellow}PID file exists (already running or unclean shutdown)${NC}\n"
        printf "$DAEMON: ${yellow}Please stop the process first. Goodbye!${NC}\n"
        exit 0
    fi

    PID=`pidof $DAEMON`
    
    if [ ! -z "$PID" ]
    then
        GOPHER=`ps -p $PID -o command h | grep gopher`
        
        if [ -z "$GOPHER" ]
        then
            printf "$DAEMON: ${magenta}Seems PID file doesn't exist, but the following process does exist: $PID${NC}\n"
            printf "$DAEMON: ${magenta}Please kill it manually. Goodbye!${NC}\n"
            exit 0
        fi
    fi
        
    cd $ROOT_DIR
    
    $ROOT_DIR/$DAEMON $ARGS &
 
    echo $! > $MASTER_PID_FILE
    printf "$DAEMON: ${green}Started with "`cat $MASTER_PID_FILE`" pid. To the moon 🚀${NC}\n"
}


# STOP
function stop()
{
    if [ -a $MASTER_PID_FILE ]
    then
        PID="`cat $MASTER_PID_FILE`"
        
        spinner "$DAEMON: Attempting to kill [Process id: $PID]" &
        
        kill $KILL_OPTION $PID
        
        sleep 1

        timeout=0
        
        while isProcessRunning $PID; do
            sleep 1
            ((timeout++))
            if [[ "$timeout" == "$TIME_OUT" ]]; then
                continue
            fi
        done
        
        if isProcessRunning $PID
        then
            printf "$DAEMON: ${blue}Process is still running at $PID. Please try "stop" command again${NC}\n"
        else
            if [ -a $MASTER_PID_FILE ]
            then
                rm $MASTER_PID_FILE
            fi
            
        fi
        
        kill "$!" # kill the spinner
    else
        PID=`pidof $DAEMON`
        
        if [ ! -z "$PID" ]
        then
            GOPHER=`ps -p $PID -o command h | grep gopher`
            
            if [ -z "$GOPHER" ]
            then
                printf "$DAEMON: ${magenta}Seems PID file doesn't exist, but the following process does exist: $PID${NC}\n"
                printf "$DAEMON: ${magenta}Please kill it manually${NC}\n"
            fi
        else
            printf "$DAEMON: ${green}Process doesn't exist. Goodbye!${NC}\n" 
        fi
    fi
}

# STATUS 
function status()
{   
    if [ -a $MASTER_PID_FILE ]
    then
        PID=$(cat $MASTER_PID_FILE)
                
        printf "$DAEMON: ${green}PID file exists at $MASTER_PID_FILE${NC}\n"
        
        if isProcessRunning $PID
        then
            printf "$DAEMON: ${green}Process is currently running at $PID${NC}\n"
        fi

        exit 0
    else
        PID=`pidof $DAEMON`
        
        if [ ! -z "$PID" ]
        then
            printf "$DAEMON: ${magenta}Seems PID file doesn't exist, but the following process does exist: $PID${NC}\n"
            printf "$DAEMON: ${magenta}This is not a clean state!${NC}\n"
        else
            printf "$DAEMON: ${red}Process is not running.${NC}\n"
        fi
        
        exit 0
    fi
}

while test $# -gt 0
do
    case "$1" in
        start)
            start && exit 0
            $1
            ;;
        stop)
            stop && exit 0
            $1
            ;;
        status)
            status
            ;;
        --*)
            echo $"Usage: $0 {start|stop|status|reload|--help|--version}"
            exit 0
            ;;
        *)
            echo $"Usage: $0 {start|stop|status|reload|--help|--version}"
            exit 0
    esac
done

exit $?
