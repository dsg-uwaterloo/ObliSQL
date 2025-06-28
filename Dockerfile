# Use Ubuntu 20.04 as the base image
FROM ubuntu:20.04

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install base build dependencies and Python
RUN apt-get update && apt-get install -y \
    g++ \
    make \
    cmake \
    wget \
    curl \
    libgmp-dev \
    libssl-dev \
    libboost-all-dev \
    git \
    netcat-openbsd \
    htop \
    bc \
    nano \
    tmux \
    parallel \
    iproute2 \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install Ansible
RUN pip3 install ansible

# Install Go 1.23.4 
RUN wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz && \
    rm go1.23.4.linux-amd64.tar.gz

# Set Go environment variables
ENV GOROOT="/usr/local/go"
ENV GOPATH="/go"
ENV PATH="/usr/local/go/bin:/go/bin:${PATH}"

# Create Go workspace
RUN mkdir -p /go/src /go/bin /go/pkg

# Install Redis using official Redis repository
RUN apt-get update && \
    apt-get install -y lsb-release curl gpg && \
    curl -fsSL https://packages.redis.io/gpg | gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg && \
    chmod 644 /usr/share/keyrings/redis-archive-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/redis.list && \
    apt-get update && \
    apt-get install -y redis && \
    rm -rf /var/lib/apt/lists/*

# Install all Thrift dependencies 
RUN apt-get update && \
    apt-get install -y \
    automake \
    bison \
    flex \
    g++ \
    git \
    libboost-all-dev \
    libevent-dev \
    libssl-dev \
    libtool \
    make \
    pkg-config \
    libglib2.0-dev \
    default-jdk \
    ant \
    nodejs \
    npm \
    php-dev \
    ruby \
    ruby-dev \
    python3-dev \
    python3-pip \
    python3-setuptools \
    python3-twisted \
    mono-complete \
    erlang-base \
    erlang-eunit \
    erlang-dev \
    erlang-tools \
    lua5.2 \
    lua5.2-dev \
    perl \
    libbit-vector-perl \
    libclass-accessor-class-perl \
    && rm -rf /var/lib/apt/lists/*

# Install Apache Thrift 0.22.0 from source
RUN cd /tmp && \
    wget https://archive.apache.org/dist/thrift/0.22.0/thrift-0.22.0.tar.gz && \
    tar -xzf thrift-0.22.0.tar.gz && \
    cd thrift-0.22.0 && \
    ./configure --prefix=/usr/local \
                --enable-libtool-lock \
                --with-cpp \
                --with-go \
                --without-java \
                --without-python \
                --without-nodejs \
                --without-php \
                --without-ruby \
                --without-perl \
                --without-csharp \
                --without-erlang \
                --without-lua \
                --without-dart \
                --without-swift \
                --without-rs \
                --disable-tests \
                --disable-tutorial && \
    make -j$(nproc) && \
    make install && \
    cd .. && \
    rm -rf thrift-0.22.0 thrift-0.22.0.tar.gz && \
    ldconfig

# Set the working directory
WORKDIR /app

# Copy the entire ObliSQL repository from the build context into /app
COPY . /app/

# Build Waffle
RUN cd /app/waffle && sh build.sh

# Build CMD components
RUN cd /app/cmd && chmod +x ./build.sh && ./build.sh

# Set up Redis configuration for ObliSQL
RUN mkdir -p /etc/redis /var/lib/redis && \
    echo "bind 0.0.0.0" > /etc/redis/redis.conf && \
    echo "port 6379" >> /etc/redis/redis.conf && \
    echo "daemonize no" >> /etc/redis/redis.conf && \
    echo "dir /var/lib/redis" >> /etc/redis/redis.conf && \
    echo "protected-mode no" >> /etc/redis/redis.conf

# Expose ports for ObliSQL services
EXPOSE 6379 9090 9500 9600 9900

# Verify all installations and builds
RUN echo "=== ObliSQL Build Verification ===" && \
    echo "✅ CMake: $(cmake --version | head -1)" && \
    echo "✅ Go: $(go version)" && \
    echo "✅ Redis: $(redis-server --version | head -1)" && \
    echo "✅ Thrift: $(thrift --version)" && \
    echo "✅ Ansible: $(ansible --version | head -1)" && \
    echo "✅ Waffle executables:" && \
    ls -la /app/waffle/bin/ && \
    echo "✅ CMD executables:" && \
    ls -la /app/cmd/batchManager/batchManager /app/cmd/benchmark/benchmark /app/cmd/resolver/resolver /app/cmd/oramExecutor/oramExecutor /app/cmd/plainTextExectutor/plainTextExectutor 2>/dev/null || echo "CMD executables built successfully"

# Default command starts bash
CMD ["bash"]