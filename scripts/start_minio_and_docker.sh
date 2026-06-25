export MINIO_ROOT_USER='zhongfabai'
export MINIO_ROOT_PASSWORD='9@olw1efacz$po!d'
nohup minio server ~/sesame/data/minio --console-address ":39091" > ~/sesame/minio.log 2>&1 &
cd ~/sesame/dockers/zookeeper && docker compose up -d
cd ~/sesame/dockers/kafka && docker compose up -d
cd ~/sesame/dockers/es && docker compose up -d
cd ~/sesame/dockers/redis && docker compose up -d