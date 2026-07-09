FROM python:3.12-slim

WORKDIR /app

COPY vip_member_gift_cloud.py /app/vip_member_gift_cloud.py

ENV VIP_GIFT_HOST=0.0.0.0 \
    VIP_GIFT_PORT=3102 \
    VIP_GIFT_DB_PATH=/data/vip_member_gift_cloud.sqlite3

VOLUME ["/data"]
EXPOSE 3102

CMD ["python", "/app/vip_member_gift_cloud.py"]
