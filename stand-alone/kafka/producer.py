from confluent_kafka import Producer
from datetime import datetime

conf = {
    'bootstrap.servers': 'localhost:9092',
    'security.protocol': 'SASL_PLAINTEXT',  # 必须明确指定
    'sasl.mechanism': 'PLAIN',
    'sasl.username': 'devuser',
    'sasl.password': 'devpass',
    'api.version.request': 'true'  # 启用版本协商
}

producer = Producer(conf)
current_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
message = f"[{current_time}] Test message"
producer.produce('test', b'hello test')
producer.flush()  # 同步发送
