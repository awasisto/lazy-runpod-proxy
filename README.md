lazy-runpod-proxy
=================

lazy-runpod-proxy is a reverse proxy designed to sit in front of a RunPod pod. It starts the pod on-demand when a
request is received and automatically stops it after a period of inactivity.

Running
-------

```bash
go mod tidy
export RUNPOD_API_KEY="your_api_key"
export POD_ID="your_pod_id"
export TARGET_BASE_URL="https://xxxxx.proxy.runpod.net"
export INACTIVITY_LIMIT_SECONDS=1200
export START_TIME_LIMIT_SECONDS=300
export RETRY_INTERVAL_SECONDS=5
export LISTEN_ADDRESS="0.0.0.0:8080"
go run main.go
```

#### With Docker

```bash
docker build -t lazy-runpod-proxy .
docker run -p 8080:8080 \
  -e RUNPOD_API_KEY="your_api_key" \
  -e POD_ID="your_pod_id" \
  -e TARGET_BASE_URL="https://xxxxx.proxy.runpod.net" \
  -e INACTIVITY_LIMIT_SECONDS=1200 \
  -e START_TIME_LIMIT_SECONDS=300 \
  -e RETRY_INTERVAL_SECONDS=5 \
  -e LISTEN_ADDRESS="0.0.0.0:8080" \
  lazy-runpod-proxy
```

License
-------

    MIT License

    Copyright (c) 2025 Andika Wasisto

    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE.