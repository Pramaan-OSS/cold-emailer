from pathlib import Path
import os,secrets,base64,subprocess,sys
root=Path(__file__).resolve().parent.parent
state=root/'.data';state.mkdir(exist_ok=True)
key=state/'local-secrets.json'
import json
if not key.exists():key.write_text(json.dumps({'AUTH_SECRET':secrets.token_hex(32),'KMS_LOCAL_MASTER_KEY':base64.b64encode(secrets.token_bytes(32)).decode(),'CREDENTIALS_ENCRYPTION_KEY':secrets.token_hex(32),'INTERNAL_API_TOKEN':secrets.token_hex(32)}));key.chmod(0o600)
env=os.environ.copy();env.update(json.loads(key.read_text()));env.update({'APP_ENV':'dev','APP_URL':'http://127.0.0.1:5417','API_HOST':'127.0.0.1:58080','API_PUBLIC_URL':'http://127.0.0.1:58080','CORS_ALLOW_ORIGINS':'http://127.0.0.1:5417','WEBSOCKET_URL':'ws://127.0.0.1:54000/socket/websocket','PRIMARY_DB':'postgres://coldemail:local-test-only@127.0.0.1:55432/coldemail?sslmode=disable','REDIS':'redis://127.0.0.1:56379','NATS_URL':'nats://127.0.0.1:54222','EVENTBUS_PROVIDER':'nats','CODEC_PROVIDER':'json','KMS_PROVIDER':'local','BLOB_PROVIDER':'filesystem','BLOB_FS_ROOT':str(state/'blobs'),'PRAMAAN_API_KEY':'fake-local-check','PRAMAAN_BASE_URL':'http://127.0.0.1:59090','EMAIL_NAME':'Cold Email','EMAIL_ADDRESS':'notifications@example.test','MAIL_TRANSPORT':'log','CAPTCHA_PROVIDER':'none','TASKS_PROVIDER':'local','BILLING_PROVIDER':'none','GEODB_PATH':str(state/'unused.mmdb'),'GIN_MODE':'release'})
env.update({'ENCRYPTED_KEYS_PROVIDER':'http' if len(sys.argv)>1 and sys.argv[1]=='worker' else 'postgres','ENCRYPTED_KEYS_BACKEND_URL':'http://127.0.0.1:58080','ENCRYPTED_KEYS_WORKER_TOKEN':env['INTERNAL_API_TOKEN'],'WORKER_PUBLIC_IP':'127.0.0.1','WORKER_STATE_DIR':str(state/'worker'),'COLD_EMAIL_NODE_NAME':'Cold Email local check','WORKER_ID':'b50a3fc8-8a4c-40ad-9910-873b7c3a7ec1'})
os.chdir(root);os.execvpe('go',['go','run','./cmd/'+(sys.argv[1] if len(sys.argv)>1 else 'backend')],env)
