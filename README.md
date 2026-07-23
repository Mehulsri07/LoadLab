# LoadLab ⚡

LoadLab is a Dockerized environment built specifically to fail on purpose. It is a dedicated system for practicing site reliability engineering, telemetry, and load testing by subjecting a robust architecture to realistic, heavy stress.

## 🚀 Overview

Understanding how systems break is the best way to make them resilient. LoadLab features an intentionally unoptimized, resource-heavy backend. By generating varying intensities of artificial traffic, developers can monitor system degradation in real-time, trace bottlenecks, and optimize infrastructure limits. 

## ✨ Key Features

* **Intentionally Expensive Endpoints:** An Nginx-proxied Node/Express backend containing routes designed to consume excessive CPU and memory.
* **Multi-Intensity Load Generation:** Configured `k6` scripts to simulate traffic at three distinct stress intensities (Low, Medium, High).
* **Real-Time Telemetry:** Fully integrated Prometheus and Grafana stack for live metrics, visualizing server strain and response times.
* **Production-Ready Deployment:** Entirely Dockerized and deployable to AWS EC2 for realistic remote load testing over the network.

## 🛠️ Tech Stack

* **Backend Environment:** Node.js, Express, Nginx
* **Load Testing:** k6
* **Monitoring & Metrics:** Prometheus, Grafana
* **Containerization & Hosting:** Docker, Docker Compose, AWS EC2

## 📦 Getting Started

1. **Clone the repository:**
   ```bash
   git clone [https://github.com/Mehulsri07/LoadLab.git](https://github.com/Mehulsri07/LoadLab.git)
