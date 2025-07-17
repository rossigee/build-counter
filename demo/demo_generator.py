#!/usr/bin/env python3
"""
Build Counter Demo Data Generator

This script generates realistic demo data for the build-counter service,
simulating a live CI/CD environment with multiple projects and builds.
"""

import os
import time
import random
import threading
import requests
import uuid
from datetime import datetime
from faker import Faker
from colorama import init, Fore, Style

# Initialize colorama for cross-platform colored output
init()

# Configuration from environment variables
BUILD_COUNTER_URL = os.getenv("BUILD_COUNTER_URL", "http://localhost:8080")
DEMO_PROJECTS = int(os.getenv("DEMO_PROJECTS", "25"))
DEMO_INTERVAL_MIN = int(os.getenv("DEMO_INTERVAL_MIN", "5"))
DEMO_INTERVAL_MAX = int(os.getenv("DEMO_INTERVAL_MAX", "30"))
DEMO_BUILD_DURATION_MIN = int(os.getenv("DEMO_BUILD_DURATION_MIN", "30"))
DEMO_BUILD_DURATION_MAX = int(os.getenv("DEMO_BUILD_DURATION_MAX", "300"))
DEMO_SUCCESS_RATE = float(os.getenv("DEMO_SUCCESS_RATE", "0.85"))

# Initialize faker
fake = Faker()

class BuildProject:
    """Represents a project that can have builds"""
    
    def __init__(self, name):
        self.name = name
        self.active_builds = {}
        self.completed_builds = 0
        self.failed_builds = 0
        
    def start_build(self):
        """Start a new build for this project"""
        build_id = str(uuid.uuid4())[:8]
        
        try:
            response = requests.post(
                f"{BUILD_COUNTER_URL}/start",
                params={"name": self.name, "build_id": build_id},
                timeout=10
            )
            
            if response.status_code == 200:
                data = response.json()
                build_info = {
                    "build_id": build_id,
                    "db_id": data.get("next_id"),
                    "start_time": datetime.now()
                }
                self.active_builds[build_id] = build_info
                
                print(f"{Fore.GREEN}✓{Style.RESET_ALL} Started build {Fore.CYAN}{self.name}#{build_id}{Style.RESET_ALL}")
                return build_id
            else:
                print(f"{Fore.RED}✗{Style.RESET_ALL} Failed to start build for {self.name}: {response.status_code}")
                
        except requests.RequestException as e:
            print(f"{Fore.RED}✗{Style.RESET_ALL} Error starting build for {self.name}: {e}")
            
        return None
    
    def finish_build(self, build_id, success=True):
        """Finish a build for this project"""
        if build_id not in self.active_builds:
            return False
            
        try:
            response = requests.post(
                f"{BUILD_COUNTER_URL}/finish",
                params={"name": self.name, "build_id": build_id},
                timeout=10
            )
            
            if response.status_code == 201:
                build_info = self.active_builds.pop(build_id)
                duration = (datetime.now() - build_info["start_time"]).total_seconds()
                
                if success:
                    self.completed_builds += 1
                    status_icon = f"{Fore.GREEN}✓{Style.RESET_ALL}"
                    status_text = f"{Fore.GREEN}completed{Style.RESET_ALL}"
                else:
                    self.failed_builds += 1
                    status_icon = f"{Fore.RED}✗{Style.RESET_ALL}"
                    status_text = f"{Fore.RED}failed{Style.RESET_ALL}"
                
                print(f"{status_icon} Build {Fore.CYAN}{self.name}#{build_id}{Style.RESET_ALL} {status_text} in {duration:.1f}s")
                return True
            else:
                print(f"{Fore.RED}✗{Style.RESET_ALL} Failed to finish build {self.name}#{build_id}: {response.status_code}")
                
        except requests.RequestException as e:
            print(f"{Fore.RED}✗{Style.RESET_ALL} Error finishing build {self.name}#{build_id}: {e}")
            
        return False

class DemoGenerator:
    """Main demo generator class"""
    
    def __init__(self):
        self.projects = {}
        self.running = True
        self.stats = {
            "total_builds_started": 0,
            "total_builds_completed": 0,
            "total_builds_failed": 0
        }
        
    def create_projects(self):
        """Create demo projects with realistic names"""
        project_types = [
            "web-{}", "api-{}", "mobile-{}", "backend-{}", "frontend-{}", 
            "service-{}", "app-{}", "platform-{}", "cli-{}", "worker-{}"
        ]
        
        tech_words = [
            "auth", "user", "payment", "notification", "analytics", "search", 
            "admin", "dashboard", "core", "gateway", "proxy", "cache", "queue",
            "monitor", "logger", "config", "deploy", "build", "test", "docs"
        ]
        
        print(f"{Fore.YELLOW}🚀 Creating {DEMO_PROJECTS} demo projects...{Style.RESET_ALL}")
        
        for i in range(DEMO_PROJECTS):
            if random.choice([True, False]):
                # Use realistic tech project names
                name_template = random.choice(project_types)
                tech_word = random.choice(tech_words)
                project_name = name_template.format(tech_word)
            else:
                # Use company/product names
                company = fake.company().lower().replace(" ", "-").replace(",", "").replace(".", "")
                project_name = f"{company}-{random.choice(['web', 'api', 'app'])}"
            
            # Ensure name is valid (alphanumeric, hyphens, underscores only)
            project_name = ''.join(c for c in project_name if c.isalnum() or c in '-_')[:50]
            
            if project_name and project_name not in self.projects:
                self.projects[project_name] = BuildProject(project_name)
                print(f"  📦 {Fore.BLUE}{project_name}{Style.RESET_ALL}")
        
        print(f"{Fore.GREEN}✓ Created {len(self.projects)} projects{Style.RESET_ALL}\n")
    
    def wait_for_service(self):
        """Wait for the build-counter service to be ready"""
        print(f"{Fore.YELLOW}⏳ Waiting for build-counter service...{Style.RESET_ALL}")
        
        max_retries = 30
        for attempt in range(max_retries):
            try:
                response = requests.get(f"{BUILD_COUNTER_URL}/health", timeout=5)
                if response.status_code == 200:
                    print(f"{Fore.GREEN}✓ Build-counter service is ready!{Style.RESET_ALL}\n")
                    return True
            except requests.RequestException:
                pass
            
            print(f"  Attempt {attempt + 1}/{max_retries}... retrying in 2s")
            time.sleep(2)
        
        print(f"{Fore.RED}✗ Failed to connect to build-counter service after {max_retries} attempts{Style.RESET_ALL}")
        return False
    
    def simulate_build_activity(self):
        """Simulate realistic build activity"""
        while self.running:
            try:
                # Select a random project
                project = random.choice(list(self.projects.values()))
                
                # Decide what to do
                if random.random() < 0.7 and len(project.active_builds) == 0:
                    # Start a new build (70% chance if no active builds)
                    build_id = project.start_build()
                    if build_id:
                        self.stats["total_builds_started"] += 1
                        
                        # Schedule this build to finish
                        duration = random.randint(DEMO_BUILD_DURATION_MIN, DEMO_BUILD_DURATION_MAX)
                        success = random.random() < DEMO_SUCCESS_RATE
                        
                        threading.Timer(
                            duration, 
                            self.finish_build_later, 
                            args=[project, build_id, success]
                        ).start()
                
                elif project.active_builds and random.random() < 0.1:
                    # Force finish a random active build (10% chance - simulates manual intervention)
                    build_id = random.choice(list(project.active_builds.keys()))
                    success = random.random() < DEMO_SUCCESS_RATE
                    if project.finish_build(build_id, success):
                        if success:
                            self.stats["total_builds_completed"] += 1
                        else:
                            self.stats["total_builds_failed"] += 1
                
                # Wait before next action
                sleep_time = random.randint(DEMO_INTERVAL_MIN, DEMO_INTERVAL_MAX)
                time.sleep(sleep_time)
                
            except Exception as e:
                print(f"{Fore.RED}✗ Error in demo loop: {e}{Style.RESET_ALL}")
                time.sleep(5)
    
    def finish_build_later(self, project, build_id, success):
        """Callback to finish a build after a delay"""
        if self.running and project.finish_build(build_id, success):
            if success:
                self.stats["total_builds_completed"] += 1
            else:
                self.stats["total_builds_failed"] += 1
    
    def print_stats(self):
        """Print periodic statistics"""
        while self.running:
            time.sleep(60)  # Print stats every minute
            
            if not self.running:
                break
                
            total_active = sum(len(p.active_builds) for p in self.projects.values())
            total_completed = sum(p.completed_builds for p in self.projects.values())
            total_failed = sum(p.failed_builds for p in self.projects.values())
            
            print(f"\n{Fore.CYAN}📊 Demo Statistics:{Style.RESET_ALL}")
            print(f"  Projects: {len(self.projects)}")
            print(f"  Active builds: {total_active}")
            print(f"  Completed builds: {total_completed}")
            print(f"  Failed builds: {total_failed}")
            print(f"  Success rate: {(total_completed / max(1, total_completed + total_failed)) * 100:.1f}%")
            print(f"  🌐 Dashboard: {Fore.BLUE}http://localhost:8080{Style.RESET_ALL}")
            print(f"  📈 Metrics: {Fore.BLUE}http://localhost:8080/metrics{Style.RESET_ALL}")
            print(f"  🔍 Grafana: {Fore.BLUE}http://localhost:3000{Style.RESET_ALL} (admin/demo)\n")
    
    def run(self):
        """Run the demo generator"""
        print(f"{Fore.MAGENTA}🎭 Build Counter Demo Generator{Style.RESET_ALL}")
        print(f"Generating demo data for: {Fore.BLUE}{BUILD_COUNTER_URL}{Style.RESET_ALL}\n")
        
        # Wait for service to be ready
        if not self.wait_for_service():
            return
        
        # Create projects
        self.create_projects()
        
        # Start statistics thread
        stats_thread = threading.Thread(target=self.print_stats, daemon=True)
        stats_thread.start()
        
        print(f"{Fore.GREEN}🎬 Starting demo simulation...{Style.RESET_ALL}")
        print(f"Visit {Fore.BLUE}http://localhost:8080{Style.RESET_ALL} to see the dashboard")
        print(f"Press Ctrl+C to stop\n")
        
        try:
            self.simulate_build_activity()
        except KeyboardInterrupt:
            print(f"\n{Fore.YELLOW}🛑 Stopping demo generator...{Style.RESET_ALL}")
            self.running = False
            
            # Finish any remaining active builds
            print("Finishing remaining active builds...")
            for project in self.projects.values():
                for build_id in list(project.active_builds.keys()):
                    project.finish_build(build_id, success=True)
            
            print(f"{Fore.GREEN}✓ Demo generator stopped{Style.RESET_ALL}")

if __name__ == "__main__":
    generator = DemoGenerator()
    generator.run()