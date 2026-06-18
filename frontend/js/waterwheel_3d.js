const Waterwheel3D = {
    scene: null,
    camera: null,
    renderer: null,
    wheel: null,
    frame: null,
    bearingMarkers: [],
    raycaster: null,
    mouse: null,
    rotationSpeed: 0.003,
    speedFactor: 1.0,
    autoRotate: true,
    animating: false,
    noriaDiameter: 8.5,
    bearings: [],
    onBearingClick: null,
    onBearingHover: null,

    init(containerId, options = {}) {
        const container = document.getElementById(containerId);
        if (!container) {
            console.error(`未找到容器 ${containerId}`);
            return;
        }

        this.noriaDiameter = options.diameter || 8.5;
        this.onBearingClick = options.onBearingClick || null;
        this.onBearingHover = options.onBearingHover || null;

        this.scene = new THREE.Scene();
        this.scene.background = new THREE.Color(0x0a1628);
        this.scene.fog = new THREE.Fog(0x0a1628, 30, 80);

        const width = container.clientWidth || 1200;
        const height = container.clientHeight || 600;

        this.camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 1000);
        this.camera.position.set(12, 10, 15);
        this.camera.lookAt(0, 0, 0);

        this.renderer = new THREE.WebGLRenderer({
            antialias: true,
            alpha: true,
        });
        this.renderer.setSize(width, height);
        this.renderer.setPixelRatio(window.devicePixelRatio || 1);
        this.renderer.shadowMap.enabled = true;
        this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
        container.appendChild(this.renderer.domElement);

        this._addLights();
        this._buildGround();
        this._buildNoriaWheel();
        this._buildFrame();
        this._buildWaterTrough();
        this._addOrbitControls();

        this.raycaster = new THREE.Raycaster();
        this.mouse = new THREE.Vector2();

        this.renderer.domElement.addEventListener("click", (e) => this._onMouseClick(e));
        this.renderer.domElement.addEventListener("mousemove", (e) => this._onMouseMove(e));

        window.addEventListener("resize", () => this._onResize(container));

        this.animate();
    },

    _addLights() {
        const ambient = new THREE.AmbientLight(0x404060, 0.6);
        this.scene.add(ambient);

        const sunLight = new THREE.DirectionalLight(0xfff8e7, 1.2);
        sunLight.position.set(10, 20, 10);
        sunLight.castShadow = true;
        sunLight.shadow.mapSize.width = 2048;
        sunLight.shadow.mapSize.height = 2048;
        sunLight.shadow.camera.near = 0.5;
        sunLight.shadow.camera.far = 50;
        sunLight.shadow.camera.left = -20;
        sunLight.shadow.camera.right = 20;
        sunLight.shadow.camera.top = 20;
        sunLight.shadow.camera.bottom = -20;
        this.scene.add(sunLight);

        const rimLight = new THREE.DirectionalLight(0x4fc3f7, 0.4);
        rimLight.position.set(-10, 5, -10);
        this.scene.add(rimLight);

        const waterLight = new THREE.PointLight(0x29b6f6, 0.8, 15);
        waterLight.position.set(0, -2, 0);
        this.scene.add(waterLight);
    },

    _buildGround() {
        const groundGeo = new THREE.CircleGeometry(40, 64);
        const groundMat = new THREE.MeshStandardMaterial({
            color: 0x1a365d,
            roughness: 0.9,
            metalness: 0.1,
        });
        const ground = new THREE.Mesh(groundGeo, groundMat);
        ground.rotation.x = -Math.PI / 2;
        ground.position.y = -3;
        ground.receiveShadow = true;
        this.scene.add(ground);

        const waterGeo = new THREE.PlaneGeometry(30, 15, 50, 30);
        const waterMat = new THREE.MeshStandardMaterial({
            color: 0x1565c0,
            transparent: true,
            opacity: 0.7,
            roughness: 0.1,
            metalness: 0.8,
            side: THREE.DoubleSide,
        });
        const water = new THREE.Mesh(waterGeo, waterMat);
        water.rotation.x = -Math.PI / 2;
        water.position.y = -2.8;
        water.position.z = -5;
        water.receiveShadow = true;
        this.scene.add(water);
        this._waterMesh = water;
    },

    _buildNoriaWheel() {
        const radius = this.noriaDiameter / 2;
        this.wheel = new THREE.Group();

        const hubGeo = new THREE.CylinderGeometry(0.5, 0.5, 2.5, 32);
        const hubMat = new THREE.MeshStandardMaterial({
            color: 0x5d4037,
            roughness: 0.7,
            metalness: 0.1,
        });
        const hub = new THREE.Mesh(hubGeo, hubMat);
        hub.rotation.z = Math.PI / 2;
        hub.castShadow = true;
        hub.receiveShadow = true;
        this.wheel.add(hub);

        const axleGeo = new THREE.CylinderGeometry(0.12, 0.12, 6, 16);
        const axleMat = new THREE.MeshStandardMaterial({
            color: 0x757575,
            roughness: 0.4,
            metalness: 0.8,
        });
        const axle = new THREE.Mesh(axleGeo, axleMat);
        axle.rotation.z = Math.PI / 2;
        axle.castShadow = true;
        this.wheel.add(axle);

        const rimGeo = new THREE.TorusGeometry(radius, 0.15, 16, 64);
        const rimMat = new THREE.MeshStandardMaterial({
            color: 0x4e342e,
            roughness: 0.8,
            metalness: 0.1,
        });
        const rim1 = new THREE.Mesh(rimGeo, rimMat);
        rim1.rotation.x = Math.PI / 2;
        rim1.position.z = 0.8;
        rim1.castShadow = true;
        this.wheel.add(rim1);

        const rim2 = rim1.clone();
        rim2.position.z = -0.8;
        this.wheel.add(rim2);

        const numSpokes = 24;
        const spokeMat = new THREE.MeshStandardMaterial({
            color: 0x6d4c41,
            roughness: 0.75,
            metalness: 0.1,
        });

        for (let i = 0; i < numSpokes; i++) {
            const angle = (i / numSpokes) * Math.PI * 2;
            const spokeGeo = new THREE.BoxGeometry(0.08, radius - 0.5, 0.08);
            const spoke = new THREE.Mesh(spokeGeo, spokeMat);
            spoke.position.y = (radius - 0.5) / 2;
            spoke.castShadow = true;

            const spokeGroup = new THREE.Group();
            spokeGroup.add(spoke);
            spokeGroup.rotation.z = angle;
            this.wheel.add(spokeGroup);

            const crossBarGeo = new THREE.BoxGeometry(1.6, 0.06, 0.06);
            const crossBar = new THREE.Mesh(crossBarGeo, spokeMat);
            crossBar.position.y = radius * 0.6;
            crossBar.castShadow = true;

            const crossGroup = new THREE.Group();
            crossGroup.add(crossBar);
            crossGroup.rotation.z = angle;
            this.wheel.add(crossGroup);
        }

        const numBuckets = 36;
        const bucketMat = new THREE.MeshStandardMaterial({
            color: 0x5d4037,
            roughness: 0.8,
            metalness: 0.1,
            side: THREE.DoubleSide,
        });

        for (let i = 0; i < numBuckets; i++) {
            const angle = (i / numBuckets) * Math.PI * 2;
            const bucketGroup = new THREE.Group();

            const bottomGeo = new THREE.BoxGeometry(0.8, 0.05, 0.4);
            const bottom = new THREE.Mesh(bottomGeo, bucketMat);
            bottom.position.y = -0.3;
            bottom.castShadow = true;
            bucketGroup.add(bottom);

            const backGeo = new THREE.BoxGeometry(0.8, 0.6, 0.05);
            const back = new THREE.Mesh(backGeo, bucketMat);
            back.position.y = 0;
            back.position.z = -0.175;
            back.castShadow = true;
            bucketGroup.add(back);

            const sideGeo = new THREE.BoxGeometry(0.05, 0.6, 0.4);
            const side1 = new THREE.Mesh(sideGeo, bucketMat);
            side1.position.x = -0.375;
            side1.position.y = 0;
            side1.castShadow = true;
            bucketGroup.add(side1);

            const side2 = side1.clone();
            side2.position.x = 0.375;
            bucketGroup.add(side2);

            const frontGeo = new THREE.BoxGeometry(0.8, 0.3, 0.05);
            const front = new THREE.Mesh(frontGeo, bucketMat);
            front.position.y = -0.15;
            front.position.z = 0.175;
            front.castShadow = true;
            bucketGroup.add(front);

            const pivotGroup = new THREE.Group();
            pivotGroup.add(bucketGroup);
            bucketGroup.position.y = radius - 0.1;
            bucketGroup.rotation.x = -Math.PI / 2.2;
            pivotGroup.rotation.z = angle;
            this.wheel.add(pivotGroup);
        }

        this.scene.add(this.wheel);
        return this.wheel;
    },

    _buildFrame() {
        this.frame = new THREE.Group();

        const postMat = new THREE.MeshStandardMaterial({
            color: 0x4e342e,
            roughness: 0.8,
            metalness: 0.1,
        });

        const postHeight = 7;
        const postGeo = new THREE.BoxGeometry(0.3, postHeight, 0.3);

        const postPositions = [
            { x: -2.5, z: -1.5 },
            { x: 2.5, z: -1.5 },
            { x: -2.5, z: 1.5 },
            { x: 2.5, z: 1.5 },
        ];

        postPositions.forEach((pos) => {
            const post = new THREE.Mesh(postGeo, postMat);
            post.position.set(pos.x, postHeight / 2 - 3, pos.z);
            post.castShadow = true;
            post.receiveShadow = true;
            this.frame.add(post);
        });

        const beamGeo = new THREE.BoxGeometry(5.6, 0.25, 0.25);
        const topBeam1 = new THREE.Mesh(beamGeo, postMat);
        topBeam1.position.set(0, postHeight - 3.2, -1.5);
        topBeam1.castShadow = true;
        this.frame.add(topBeam1);

        const topBeam2 = topBeam1.clone();
        topBeam2.position.z = 1.5;
        this.frame.add(topBeam2);

        const crossBeamGeo = new THREE.BoxGeometry(0.25, 0.25, 3.4);
        const cross1 = new THREE.Mesh(crossBeamGeo, postMat);
        cross1.position.set(-2.5, postHeight - 3.2, 0);
        cross1.castShadow = true;
        this.frame.add(cross1);

        const cross2 = cross1.clone();
        cross2.position.x = 2.5;
        this.frame.add(cross2);

        const braceGeo = new THREE.BoxGeometry(0.15, 4, 0.15);
        const bracePositions = [
            { x: -2, z: -1, rx: 0.4, rz: -0.2 },
            { x: 2, z: -1, rx: 0.4, rz: 0.2 },
            { x: -2, z: 1, rx: -0.4, rz: -0.2 },
            { x: 2, z: 1, rx: -0.4, rz: 0.2 },
        ];

        bracePositions.forEach((pos) => {
            const brace = new THREE.Mesh(braceGeo, postMat);
            brace.position.set(pos.x, postHeight / 2 - 3, pos.z);
            brace.rotation.x = pos.rx;
            brace.rotation.z = pos.rz;
            brace.castShadow = true;
            this.frame.add(brace);
        });

        this.scene.add(this.frame);
    },

    _buildWaterTrough() {
        const troughGroup = new THREE.Group();

        const troughMat = new THREE.MeshStandardMaterial({
            color: 0x5d4037,
            roughness: 0.8,
            metalness: 0.1,
        });

        const bottomGeo = new THREE.BoxGeometry(5, 0.15, 2.5);
        const bottom = new THREE.Mesh(bottomGeo, troughMat);
        bottom.position.set(0, -2.7, 5);
        bottom.receiveShadow = true;
        bottom.castShadow = true;
        troughGroup.add(bottom);

        const sideGeo = new THREE.BoxGeometry(5, 0.5, 0.1);
        const side1 = new THREE.Mesh(sideGeo, troughMat);
        side1.position.set(0, -2.5, 6.2);
        side1.castShadow = true;
        troughGroup.add(side1);

        const side2 = side1.clone();
        side2.position.z = 3.8;
        troughGroup.add(side2);

        const endGeo = new THREE.BoxGeometry(0.1, 0.5, 2.5);
        const end1 = new THREE.Mesh(endGeo, troughMat);
        end1.position.set(-2.45, -2.5, 5);
        end1.castShadow = true;
        troughGroup.add(end1);

        const end2 = end1.clone();
        end2.position.x = 2.45;
        troughGroup.add(end2);

        this.scene.add(troughGroup);
    },

    setBearings(bearings) {
        this.bearings = bearings;
        this._clearBearingMarkers();
        bearings.forEach((bearing, index) => {
            this._addBearingMarker(bearing, index);
        });
    },

    _clearBearingMarkers() {
        this.bearingMarkers.forEach((marker) => {
            this.scene.remove(marker);
        });
        this.bearingMarkers = [];
    },

    _addBearingMarker(bearing, index) {
        const positions = [
            { x: -3.1, y: 0.8, z: -1.6 },
            { x: 3.1, y: 0.8, z: -1.6 },
            { x: -3.1, y: 0.8, z: 1.6 },
        ];

        const pos = positions[index % positions.length];

        const markerGroup = new THREE.Group();
        markerGroup.userData = { bearing, isMarker: true };

        const housingGeo = new THREE.BoxGeometry(0.7, 0.5, 0.7);
        const housingMat = new THREE.MeshStandardMaterial({
            color: 0x757575,
            roughness: 0.5,
            metalness: 0.7,
            emissive: 0x1a237e,
            emissiveIntensity: 0.3,
        });
        const housing = new THREE.Mesh(housingGeo, housingMat);
        housing.castShadow = true;
        housing.receiveShadow = true;
        markerGroup.add(housing);

        const ringGeo = new THREE.TorusGeometry(0.25, 0.05, 12, 32);
        const ringMat = new THREE.MeshStandardMaterial({
            color: 0xb71c1c,
            emissive: 0xb71c1c,
            emissiveIntensity: 0.5,
            roughness: 0.3,
            metalness: 0.9,
        });
        const ring = new THREE.Mesh(ringGeo, ringMat);
        ring.rotation.y = Math.PI / 2;
        ring.position.x = 0.36;
        markerGroup.add(ring);

        const glowGeo = new THREE.SphereGeometry(0.5, 16, 16);
        const glowMat = new THREE.MeshBasicMaterial({
            color: 0x4fc3f7,
            transparent: true,
            opacity: 0,
        });
        const glow = new THREE.Mesh(glowGeo, glowMat);
        glow.name = "glow";
        markerGroup.add(glow);

        markerGroup.position.set(pos.x, pos.y, pos.z);
        this.scene.add(markerGroup);
        this.bearingMarkers.push(markerGroup);
    },

    updateBearingHealth(bearingId, status) {
        this.bearingMarkers.forEach((marker) => {
            if (marker.userData.bearing && marker.userData.bearing.id === bearingId) {
                const housing = marker.children[0];
                const ring = marker.children[1];

                let ringColor, emissiveIntensity;
                switch (status) {
                    case "normal":
                        ringColor = 0x66bb6a;
                        emissiveIntensity = 0.4;
                        break;
                    case "warning":
                        ringColor = 0xffa726;
                        emissiveIntensity = 0.6;
                        break;
                    case "critical":
                        ringColor = 0xef5350;
                        emissiveIntensity = 0.8;
                        break;
                    default:
                        ringColor = 0x9e9e9e;
                        emissiveIntensity = 0.2;
                }

                ring.material.color.setHex(ringColor);
                ring.material.emissive.setHex(ringColor);
                ring.material.emissiveIntensity = emissiveIntensity;
            }
        });
    },

    _addOrbitControls() {
        this._isDragging = false;
        this._previousMousePosition = { x: 0, y: 0 };
        this._spherical = {
            radius: 20,
            theta: Math.PI / 4,
            phi: Math.PI / 3,
        };

        const dom = this.renderer.domElement;

        dom.addEventListener("mousedown", (e) => {
            if (e.button === 0) {
                this._isDragging = true;
                this._previousMousePosition = { x: e.clientX, y: e.clientY };
            }
        });

        window.addEventListener("mouseup", () => {
            this._isDragging = false;
        });

        window.addEventListener("mousemove", (e) => {
            if (!this._isDragging) return;

            const deltaX = e.clientX - this._previousMousePosition.x;
            const deltaY = e.clientY - this._previousMousePosition.y;

            this._spherical.theta -= deltaX * 0.005;
            this._spherical.phi = Math.max(
                0.1,
                Math.min(Math.PI - 0.1, this._spherical.phi - deltaY * 0.005)
            );

            this._updateCameraPosition();
            this._previousMousePosition = { x: e.clientX, y: e.clientY };
        });

        dom.addEventListener("wheel", (e) => {
            e.preventDefault();
            this._spherical.radius = Math.max(
                5,
                Math.min(50, this._spherical.radius * (1 + e.deltaY * 0.001))
            );
            this._updateCameraPosition();
        }, { passive: false });

        this._updateCameraPosition();
    },

    _updateCameraPosition() {
        const { radius, theta, phi } = this._spherical;
        this.camera.position.x = radius * Math.sin(phi) * Math.sin(theta);
        this.camera.position.y = radius * Math.cos(phi);
        this.camera.position.z = radius * Math.sin(phi) * Math.cos(theta);
        this.camera.lookAt(0, 0, 0);
    },

    resetView() {
        this._spherical = {
            radius: 20,
            theta: Math.PI / 4,
            phi: Math.PI / 3,
        };
        this._updateCameraPosition();
    },

    setRotationSpeed(factor) {
        this.speedFactor = factor;
    },

    setAutoRotate(enabled) {
        this.autoRotate = enabled;
    },

    setWheelRPM(rpm) {
        if (rpm && rpm > 0) {
            this.rotationSpeed = (rpm * 2 * Math.PI) / 60 / 60;
        }
    },

    _onMouseClick(event) {
        if (!this.renderer) return;

        const rect = this.renderer.domElement.getBoundingClientRect();
        this.mouse.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
        this.mouse.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;

        this.raycaster.setFromCamera(this.mouse, this.camera);
        const intersects = this.raycaster.intersectObjects(this.bearingMarkers, true);

        if (intersects.length > 0) {
            let obj = intersects[0].object;
            while (obj && !obj.userData.isMarker) {
                obj = obj.parent;
            }
            if (obj && obj.userData.bearing && this.onBearingClick) {
                this.onBearingClick(obj.userData.bearing);
            }
        }
    },

    _onMouseMove(event) {
        if (!this.renderer) return;

        const rect = this.renderer.domElement.getBoundingClientRect();
        this.mouse.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
        this.mouse.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;

        this.raycaster.setFromCamera(this.mouse, this.camera);
        const intersects = this.raycaster.intersectObjects(this.bearingMarkers, true);

        this.bearingMarkers.forEach((marker) => {
            const glow = marker.getObjectByName("glow");
            if (glow) {
                glow.material.opacity = 0;
            }
        });

        if (intersects.length > 0) {
            let obj = intersects[0].object;
            while (obj && !obj.userData.isMarker) {
                obj = obj.parent;
            }
            if (obj) {
                const glow = obj.getObjectByName("glow");
                if (glow) {
                    glow.material.opacity = 0.3;
                }
                if (this.onBearingHover) {
                    this.onBearingHover(obj.userData.bearing);
                }
                this.renderer.domElement.style.cursor = "pointer";
            }
        } else {
            this.renderer.domElement.style.cursor = "default";
        }
    },

    _onResize(container) {
        if (!container || !this.renderer || !this.camera) return;

        const width = container.clientWidth;
        const height = container.clientHeight;

        this.camera.aspect = width / height;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(width, height);
    },

    animate() {
        this.animating = true;

        const render = () => {
            if (!this.animating) return;

            requestAnimationFrame(render);

            if (this.wheel && this.autoRotate) {
                this.wheel.rotation.z += this.rotationSpeed * this.speedFactor;
            }

            if (this._waterMesh) {
                const time = Date.now() * 0.001;
                const positions = this._waterMesh.geometry.attributes.position;
                for (let i = 0; i < positions.count; i++) {
                    const x = positions.getX(i);
                    const z = positions.getZ(i);
                    const wave =
                        Math.sin(x * 0.5 + time) * 0.05 +
                        Math.cos(z * 0.7 + time * 1.3) * 0.03;
                    positions.setY(i, wave);
                }
                positions.needsUpdate = true;
                this._waterMesh.geometry.computeVertexNormals();
            }

            this.bearingMarkers.forEach((marker, index) => {
                const time = Date.now() * 0.002;
                const ring = marker.children[1];
                if (ring) {
                    ring.material.emissiveIntensity =
                        0.4 + Math.sin(time + index) * 0.2;
                }
            });

            this.renderer.render(this.scene, this.camera);
        };

        render();
    },

    destroy() {
        this.animating = false;
        if (this.renderer) {
            this.renderer.dispose();
        }
    },
};
