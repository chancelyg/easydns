#!/bin/bash

URL="https://raw.githubusercontent.com/felixonmars/dnsmasq-china-list/master/accelerated-domains.china.conf"
OUTPUT_FILE="domestic-domain.txt"

# 创建输出文件（如果文件存在则清空内容）
> "$OUTPUT_FILE"

# 使用 curl 获取文件内容并处理
curl -s "$URL" | while read -r line; do
    # 从每一行中提取域名部分
    if [[ $line =~ ^server=/(.*)/.*$ ]]; then
        DOMAIN="${BASH_REMATCH[1]}"
        echo "$DOMAIN" >> "$OUTPUT_FILE"
        echo "$DOMAIN has been added."
    fi
done

echo "$OUTPUT_FILE file has been generated."