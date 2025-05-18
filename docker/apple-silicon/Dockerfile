# Dockerfile.opencv-arm64 - Image for Apple Silicon (M-series) with OpenCV
# This Dockerfile is optimized for ARM64 architecture with Metal support
# Designed for Apple Silicon (M1, M2, M3) processors

# Stage 1: OpenCV Builder for ARM64
FROM --platform=linux/arm64 ubuntu:22.04 AS opencv-builder

# Install OpenCV dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    git \
    pkg-config \
    libgtk2.0-dev \
    libavcodec-dev \
    libavformat-dev \
    libswscale-dev \
    libtbb2 \
    libtbb-dev \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    libdc1394-22-dev \
    libv4l-dev \
    libopenblas-dev \
    libatlas-base-dev \
    libblas-dev \
    liblapack-dev \
    libhdf5-serial-dev \
    libprotobuf-dev \
    protobuf-compiler \
    libgflags-dev \
    libgoogle-glog-dev \
    libgtk-3-dev \
    wget \
    unzip \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Set OpenCV version to match gocv 0.36.0
ENV OPENCV_VERSION="4.8.0"

# Download and extract OpenCV and contrib modules
RUN cd /tmp && \
    wget -q https://github.com/opencv/opencv/archive/${OPENCV_VERSION}.zip && \
    unzip ${OPENCV_VERSION}.zip && \
    rm ${OPENCV_VERSION}.zip && \
    wget -q https://github.com/opencv/opencv_contrib/archive/${OPENCV_VERSION}.zip && \
    unzip ${OPENCV_VERSION}.zip && \
    rm ${OPENCV_VERSION}.zip

# Build OpenCV with ARM optimizations and contrib modules
RUN cd /tmp/opencv-${OPENCV_VERSION} && mkdir build && cd build && \
    cmake -D CMAKE_BUILD_TYPE=RELEASE \
          -D CMAKE_INSTALL_PREFIX=/usr/local \
          -D OPENCV_EXTRA_MODULES_PATH=/tmp/opencv_contrib-${OPENCV_VERSION}/modules \
          -D ENABLE_NEON=ON \
          -D WITH_OPENMP=ON \
          -D BUILD_TESTS=OFF \
          -D BUILD_PERF_TESTS=OFF \
          -D BUILD_EXAMPLES=OFF \
          -D BUILD_DOCS=OFF \
          -D INSTALL_PYTHON_EXAMPLES=OFF \
          -D INSTALL_C_EXAMPLES=OFF \
          -D WITH_FFMPEG=ON \
          -D WITH_TBB=ON \
          -D BUILD_opencv_java=OFF \
          -D BUILD_opencv_python=OFF \
          -D BUILD_opencv_python2=OFF \
          -D BUILD_opencv_python3=OFF \
          -D OPENCV_GENERATE_PKGCONFIG=ON \
          -D WITH_CUDA=OFF \
          -D WITH_OPENCL=OFF \
          -D OPENCV_ENABLE_NONFREE=ON \
          .. && \
    make -j$(nproc) && \
    make install && \
    ldconfig

# Stage 2: Go Builder with OpenCV for ARM64
FROM --platform=linux/arm64 golang:1.24-bullseye AS builder

# Copy OpenCV libraries from the opencv-builder stage
COPY --from=opencv-builder /usr/local/include/opencv4 /usr/local/include/opencv4
COPY --from=opencv-builder /usr/local/lib /usr/local/lib
COPY --from=opencv-builder /usr/local/share/opencv4 /usr/local/share/opencv4
COPY --from=opencv-builder /usr/local/bin /usr/local/bin
COPY --from=opencv-builder /usr/local/lib/pkgconfig/opencv4.pc /usr/local/lib/pkgconfig/opencv4.pc

# Install dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libgtk-3-dev \
    ffmpeg \
    libavcodec-dev \
    libavutil-dev \
    libswscale-dev \
    libavformat-dev \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    ca-certificates \
    git \
    tzdata \
    libtbb2 \
    && rm -rf /var/lib/apt/lists/*

# Update ldconfig to include OpenCV libraries
RUN ldconfig

WORKDIR /app

# First, copy only go.mod and go.sum to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the application with CGO enabled for OpenCV
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o double-take ./cmd/server

# Stage 3: Runtime image for ARM64
FROM --platform=linux/arm64 ubuntu:22.04

WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    mosquitto-clients \
    jq \
    curl \
    libtbb2 \
    libgtk-3-0 \
    libavcodec58 \
    libavformat58 \
    libswscale5 \
    libjpeg8 \
    libpng16-16 \
    libtiff5 \
    && rm -rf /var/lib/apt/lists/*

# Copy OpenCV libraries from the opencv-builder stage
COPY --from=opencv-builder /usr/local/lib/libopencv_*.so* /usr/local/lib/
COPY --from=opencv-builder /usr/local/share/opencv4 /usr/local/share/opencv4

# Update ldconfig to include OpenCV libraries
RUN ldconfig

# Set timezone data
ENV TZ=Europe/Berlin

# Copy the binary from the builder stage
COPY --from=builder /app/double-take /app/double-take

# Copy web files and other necessary assets
COPY --from=builder /app/web /app/web

# Create required directories
RUN mkdir -p /data/snapshots /config

# Define volumes
VOLUME ["/data", "/config"]

# Expose the web interface port
EXPOSE 3000

# Start the application
CMD ["/app/double-take"]
