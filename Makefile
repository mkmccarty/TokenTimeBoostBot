# --------------------------------------------------
#   Makefile for Discord Bot (Prod + Debug)
# --------------------------------------------------

PROD_IMAGE=my-bot
DEBUG_IMAGE=my-bot-debug
CONFIG_FILE=$(PWD)/.config.json

all: prod

# --------------------------------------------------
#   Build images
# --------------------------------------------------

build-prod:
	docker build -t $(PROD_IMAGE) .

build-debug:
	docker build -t $(DEBUG_IMAGE) .

# --------------------------------------------------
#   Run containers
# --------------------------------------------------

prod: build-prod
	@echo "🚀 Starting production container..."
	-docker stop $(PROD_IMAGE) 2>/dev/null || true
	-docker rm $(PROD_IMAGE) 2>/dev/null || true
	docker run -d --name $(PROD_IMAGE) \
		--read-only \
		--pids-limit=200 \
		--memory=128m \
		--cap-drop=ALL \
		--tmpfs /tmp:rw,nosuid,nodev,noexec,size=16m \
		--restart unless-stopped \
		-v $(PWD)/.config.json:/app/.config.json:ro \
		-v $(PWD)/banners:/app/banners:rw \
		-v $(PWD)/ttbb-data:/app/ttbb-data:rw \
		$(PROD_IMAGE)

debug: build-debug
	@echo "🐞 Rebuilding + running debug container (Delve on :4000)..."
	-docker stop $(DEBUG_IMAGE) 2>/dev/null || true
	-docker rm $(DEBUG_IMAGE) 2>/dev/null || true
	docker run -d --name $(DEBUG_IMAGE) -p 4000:4000 \
		--pids-limit=300 \
		--memory=256m \
		--cap-drop=ALL \
		--tmpfs /tmp:rw,nosuid,nodev,noexec,size=32m \
		--restart unless-stopped \
		-v $(PWD)/.config.json:/app/.config.json:ro \
		-v $(PWD)/banners:/app/banners:rw \
		-v $(PWD)/ttbb-data:/app/ttbb-data:rw \
		$(DEBUG_IMAGE)

# --------------------------------------------------
#   Stop & cleanup
# --------------------------------------------------

stop:
	@echo "🛑 Stopping containers..."
	-docker stop $(PROD_IMAGE) $(DEBUG_IMAGE) 2>/dev/null || true
	-docker rm $(PROD_IMAGE) $(DEBUG_IMAGE) 2>/dev/null || true

clean: stop
	@echo "🧹 Cleaning up Docker resources..."
	docker system prune -af

# --------------------------------------------------
#   Rebuild (clean + debug)
# --------------------------------------------------

rebuild: clean
	@echo "🔁 Full clean rebuild of debug container..."
	make debug

# --------------------------------------------------
#   Logs
# --------------------------------------------------

logs-prod:
	docker logs -f $(PROD_IMAGE)

logs-debug:
	docker logs -f $(DEBUG_IMAGE)
