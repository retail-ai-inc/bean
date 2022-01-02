{
    "beanVersion": "{{ .BeanVersion }}",
    "packagePath": "{{ .PkgPath }}",
    "projectName": "{{ .PkgName }}",
    "projectVersion": "1.0",
    "environment": "local",
    "isLogStdout": false,
    "logFile": "logs/console.log",
    "isBodyDump": true,
    "prometheus": {
        "isPrometheusMetrics": false,
        "skipEndpoints": ["/ping", "/route/stats"]
    },
    "http": {
        "port": 8888,
        "host": "0.0.0.0",
        "bodyLimit": "1M",
        "isHttpsRedirect": false,
        "allowedMethod": ["DELETE", "GET", "POST", "PUT"],
        "uriLatencyIntervals": [5, 10, 15],
        "timeout": 24
    },
    "database": {
        "mysql": {
            "isTenant": false,
            "master":{
                "database": "naviee_master",
                "username": "master",
                "password": "secret",
                "host": "127.0.0.1",
                "port": "3306"
            },
            "maxIdleConnections": 20,
            "maxOpenConnections": 30,
            "debug": true
        },
        "mongo": {
            "master": {},
            "connectTimeout": 10
        },
        "redis": {
            "master": {},
            "prefix": "bean_cache",
            "maxretries": 2
        },
        "badger": {
            "dir": "",
            "inMemory": true
        }
    },
    "queue": {
        "redis": {
            "password": "64vc7632-62dc-482e-67fg-046c7faec493",
            "host": "127.0.0.1",
            "port": 6379,
            "name": 3,
            "prefix": "bean_queue",
            "poolsize": 100,
            "maxidle": 2
        },
        "health": {
            "port": 7777,
            "host": "0.0.0.0"
        }
    },
    "jwt": {
        "expirationSeconds": 86400,
        "secret": "ScV5nHaw2fKUZzDsgXHmS35d"
    },
    "sentry": {
        "isSentry": false,
        "dsn": "",
        "attachStacktrace": true,
        "apmTracesSampleRate": 0.0
    },
    "security": {
        "http": {
            "header": {
                "xssProtection": "1; mode=block",
                "contentTypeNosniff": "nosniff",
                "xFrameOptions": "SAMEORIGIN",
                "hstsMaxAge": 31536000,
                "contentSecurityPolicy": ""
            }
        }
    }
}