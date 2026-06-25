# SesameSauce 生产部署指南

## 前置依赖

服务器需安装：
- **Nginx**
- **Node.js**（仅构建时需要，构建后可移除）
- **Go**（运行后端服务）

```bash
# Ubuntu/Debian 安装 Nginx
sudo apt update
sudo apt install -y nginx

# 验证安装及目录结构
nginx -v
ls -la /etc/nginx/
```

---

## 1. 前端构建

在本地或服务器上执行：

```bash
cd frontend
npm install
npm run build
```

构建成功后，会生成 `frontend/dist/` 目录。

---

## 2. 上传构建产物到服务器

将 `frontend/dist` 目录上传到服务器的 `/var/www/sesame-sauce/frontend/dist`：

**方式一：scp（本地执行）**
``` bash
root@iZuf6ey58fk9jk43uw5499Z:~/sesame/Sesame-sauce/frontend# mkdir -p /var/www/sesame-sauce/frontend
root@iZuf6ey58fk9jk43uw5499Z:~/sesame/Sesame-sauce/frontend# cp -r /root/sesame/Sesame-sauce/frontend/dist /var/www/sesame-sauce/frontend/
``` 
```bash
# 在本地项目根目录执行
scp -r frontend/dist root@<服务器IP>:/var/www/sesame-sauce/frontend/
```

**方式二：rsync（推荐，支持增量同步）**

```bash
# 在本地项目根目录执行
rsync -avz --delete frontend/dist/ root@<服务器IP>:/var/www/sesame-sauce/frontend/dist/
```

**方式三：若已在服务器上构建，直接复制**

```bash
# 在服务器上执行
mkdir -p /var/www/sesame-sauce/frontend
cp -r /root/sesame/Sesame-sauce/frontend/dist /var/www/sesame-sauce/frontend/
```

> 若路径不同，请同步修改 `deploy/nginx.conf` 中的 `root` 指令。

---

## 3. 部署 Nginx 配置

```bash
# 上传配置文件
cp deploy/nginx.conf /etc/nginx/sites-available/sesame-sauce

# 启用站点
ln -sf /etc/nginx/sites-available/sesame-sauce /etc/nginx/sites-enabled/sesame-sauce

# 检查配置语法
nginx -t

# 重载 Nginx
sudo systemctl reload nginx
```

---

## 4. 启动后端服务

```bash
cd backend
go run cmd/api/main.go
```

建议后端使用 **systemd** 或 **supervisor** 守护运行，避免 SSH 断开导致服务停止。

---

## 5. 防火墙放行

```bash
# 放行 HTTP 80 端口
sudo ufw allow 80/tcp

# 如果开启了 HTTPS（后续配置 SSL），也需放行 443
sudo ufw allow 443/tcp

# 后端 8080 端口无需对外开放，Nginx 通过本机回环 127.0.0.1 访问即可
```

---

## 6. 验证部署

1. 浏览器访问 `http://<服务器IP>/`
2. 确认页面正常加载
3. 测试登录/注册等 API 功能，确认 `/api` 请求正常转发到后端

---

## 目录结构参考

```
/var/www/sesame-sauce/
└── frontend/
    └── dist/          # 前端构建产物

/etc/nginx/
├── nginx.conf
├── sites-available/
│   └── sesame-sauce   # 本配置文件
└── sites-enabled/
    └── sesame-sauce -> ../sites-available/sesame-sauce
```
