#!/bin/bash

# 配置
OSS_PATH="oss://df-patch-no-delete/patch/6.6/6.6.9/latest/"
DOWNLOAD_DIR="/home/latest_patch_enhanced/patch_tar_gz"
IMAGE_LIST_DIR="/home/latest_patch_enhanced/patch_image_list"
LOG_FILE="/var/log/patch_processor.log"
MAX_RETRIES=3
RETRY_DELAY=30

# 日志函数
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

# 错误处理函数
error_exit() {
    log "错误: $1"
    exit 1
}

# 创建目录
mkdir -p "$DOWNLOAD_DIR" "$IMAGE_LIST_DIR"
mkdir -p $(dirname "$LOG_FILE")

log "开始处理最新补丁"

# 检查ossutil
if ! command -v ossutil &> /dev/null; then
    error_exit "ossutil 未安装"
fi

# 获取最新文件
for ((i=1; i<=MAX_RETRIES; i++)); do
    LATEST_FILE=$(ossutil ls "$OSS_PATH" 2>> "$LOG_FILE" | grep -i "\.tar\.gz$" | tail -1 | awk '{print $NF}')
    
    if [ -n "$LATEST_FILE" ]; then
        break
    fi
    
    if [ $i -eq $MAX_RETRIES ]; then
        error_exit "无法获取文件列表，重试次数已达上限"
    fi
    
    log "第 $i 次获取文件列表失败，${RETRY_DELAY}秒后重试..."
    sleep $RETRY_DELAY
done

log "找到最新文件: $LATEST_FILE"

# 提取文件名（不含扩展名）
FILENAME=$(basename "$LATEST_FILE" .tar.gz)
TAR_FILE="$FILENAME.tar.gz"
LOCAL_TAR_PATH="$DOWNLOAD_DIR/$TAR_FILE"

# 检查是否已处理过
IMAGE_LIST_TARGET_DIR="$IMAGE_LIST_DIR/${FILENAME}"
if [ -d "$IMAGE_LIST_TARGET_DIR" ]; then
    log "该文件已处理过，跳过: $FILENAME"
    exit 0
fi

# 下载文件
if [ -f "$LOCAL_TAR_PATH" ]; then
    log "文件已存在，跳过下载: $TAR_FILE"
else
    for ((i=1; i<=MAX_RETRIES; i++)); do
        log "开始下载 (尝试 $i/$MAX_RETRIES): $TAR_FILE"
        
        if ossutil cp "$LATEST_FILE" "$LOCAL_TAR_PATH" >> "$LOG_FILE" 2>&1; then
            log "下载成功: $TAR_FILE"
            break
        fi
        
        if [ $i -eq $MAX_RETRIES ]; then
            error_exit "下载失败，重试次数已达上限"
        fi
        
        log "下载失败，${RETRY_DELAY}秒后重试..."
        sleep $RETRY_DELAY
        rm -f "$LOCAL_TAR_PATH"
    done
fi

# 第一次解压 - 在DOWNLOAD_DIR中解压
log "开始第一次解压: $TAR_FILE"
cd "$DOWNLOAD_DIR"

if ! tar -xf "$TAR_FILE"; then
    error_exit "第一次解压失败: $TAR_FILE"
fi

# 检查第一次解压结果
log "第一次解压完成，查看生成的文件:"
ls -la "$DOWNLOAD_DIR/" | grep "$FILENAME" >> "$LOG_FILE"

# 第二次解压 - 解压内部同名的tar.gz文件
INNER_TAR_PATH="$DOWNLOAD_DIR/$FILENAME/$TAR_FILE"
if [ -f "$INNER_TAR_PATH" ]; then
    log "开始第二次解压内部文件: $INNER_TAR_PATH"
    
    # 进入第一次解压的目录进行第二次解压
    cd "$DOWNLOAD_DIR/$FILENAME"
    
    if ! tar -xf "$TAR_FILE"; then
        error_exit "第二次解压失败: $TAR_FILE"
    fi
    log "第二次解压完成"
    
    # 查看第二次解压结果
    log "第二次解压后目录内容:"
    ls -la >> "$LOG_FILE"
else
    error_exit "未找到内部tar文件: $INNER_TAR_PATH"
fi

# 查找目标文件
TARGET_FILE_PATH="$DOWNLOAD_DIR/$FILENAME/6.6/6.6.9/$FILENAME/patch_image_tag_list.txt"
if [ -f "$TARGET_FILE_PATH" ]; then
    log "找到目标文件: $TARGET_FILE_PATH"
    
    # 创建目标目录
    mkdir -p "$IMAGE_LIST_TARGET_DIR"
    
    # 复制文件
    if cp "$TARGET_FILE_PATH" "$IMAGE_LIST_TARGET_DIR/"; then
        log "成功复制 patch_image_tag_list.txt 到 $IMAGE_LIST_TARGET_DIR/"
        
        # 可选：复制其他相关文件
        #SOURCE_DIR="$DOWNLOAD_DIR/$FILENAME/6.6/6.6.9/$FILENAME"
        #if [ -f "$SOURCE_DIR/download_extra.sh" ]; then
        #    cp "$SOURCE_DIR/download_extra.sh" "$IMAGE_LIST_TARGET_DIR/"
        #    log "复制 download_extra.sh"
        #fi
        
        # 显示最终结果
        log "处理完成，最终文件列表:"
        ls -la "$IMAGE_LIST_TARGET_DIR/" >> "$LOG_FILE"
        
    else
        error_exit "复制文件失败"
    fi
else
    error_exit "未找到目标文件: $TARGET_FILE_PATH"
fi

log "处理完成: $FILENAME"
log "文件已保存到: $IMAGE_LIST_TARGET_DIR/"