# gamja: pinned commit, runtime dependencies only. Bump GAMJA_COMMIT to upgrade.
FROM node:22-alpine@sha256:0e1754758c2df4cfa3e68d59e2e1b7374bb7fd9ea6328eaaacf15cc934af8202 AS gamja
ARG GAMJA_COMMIT=59ed8b13efadc4753efc29afb8e9437bd3df3875
RUN apk add --no-cache git
WORKDIR /src
RUN git init -q \
 && git remote add origin https://codeberg.org/emersion/gamja.git \
 && git fetch -q --depth 1 origin "$GAMJA_COMMIT" \
 && git checkout -q FETCH_HEAD \
 && npm ci --omit=dev --ignore-scripts
RUN mkdir /out \
 && cp -r index.html main.js commands.js keybindings.js state.js store.js style.css manifest.json LICENSE \
       components lib node_modules /out/
COPY gamja-config.json /out/config.json
COPY gamja-tabs.js /out/tabs.js
# Load tabs.js before gamja. The grep fails the build if a gamja upgrade changes this line.
RUN sed -i 's|<script type="module" src="./main.js"></script>|<script src="./tabs.js"></script>&|' /out/index.html \
 && grep -q 'src="./tabs.js"' /out/index.html

FROM golang:1.27-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go favicon.png ./
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /splash .

FROM scratch
COPY --from=build /splash /splash
COPY --from=gamja /out /gamja
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/splash"]
