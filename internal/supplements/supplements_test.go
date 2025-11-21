package supplements

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

// =============================================================================
// Registry Tests
// =============================================================================

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}

	supplements := r.GetAll()
	expectedCount := 6 // Express, FastAPI, Gin, SpringBoot, Django, NestJS

	if len(supplements) != expectedCount {
		t.Errorf("expected %d supplements, got %d", expectedCount, len(supplements))
	}
}

func TestRegistry_GetAll(t *testing.T) {
	r := NewRegistry()
	supplements := r.GetAll()

	expectedNames := []string{"express", "fastapi", "gin", "springboot", "django", "nestjs"}

	for _, expName := range expectedNames {
		found := false
		for _, s := range supplements {
			if s.Name() == expName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("supplement %s not found", expName)
		}
	}
}

func TestRegistry_Register(t *testing.T) {
	r := &Registry{supplements: nil}

	if len(r.GetAll()) != 0 {
		t.Error("initial registry should be empty")
	}

	r.Register(&ExpressSupplement{})
	if len(r.GetAll()) != 1 {
		t.Errorf("expected 1 supplement, got %d", len(r.GetAll()))
	}

	r.Register(&FastAPISupplement{})
	if len(r.GetAll()) != 2 {
		t.Errorf("expected 2 supplements, got %d", len(r.GetAll()))
	}
}

func TestRegistry_Detect(t *testing.T) {
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	// Create Express project files
	createFile(t, tmpDir, "package.json", `{"dependencies": {"express": "^4.0.0"}}`)
	createFile(t, tmpDir, "app.js", `const express = require('express');`)

	r := NewRegistry()
	files := []string{
		filepath.Join(tmpDir, "package.json"),
		filepath.Join(tmpDir, "app.js"),
	}

	detected := r.Detect(files)
	if len(detected) == 0 {
		t.Error("should detect at least Express supplement")
	}

	foundExpress := false
	for _, s := range detected {
		if s.Name() == "express" {
			foundExpress = true
			break
		}
	}
	if !foundExpress {
		t.Error("Express supplement should be detected")
	}
}

func TestRegistry_Detect_Empty(t *testing.T) {
	r := NewRegistry()

	// Non-existent files should not trigger any detection
	detected := r.Detect([]string{"nonexistent.xyz"})
	if len(detected) != 0 {
		t.Errorf("expected 0 supplements for unknown files, got %d", len(detected))
	}

	// Empty file list
	detected = r.Detect([]string{})
	if len(detected) != 0 {
		t.Errorf("expected 0 supplements for empty list, got %d", len(detected))
	}
}

// =============================================================================
// Express Supplement Tests
// =============================================================================

func TestExpressSupplement_Name(t *testing.T) {
	s := &ExpressSupplement{}
	if s.Name() != "express" {
		t.Errorf("Name() = %s, want express", s.Name())
	}
}

func TestExpressSupplement_Detect_PackageJSON(t *testing.T) {
	s := &ExpressSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "express in dependencies",
			content: `{"dependencies": {"express": "^4.18.0"}}`,
			want:    true,
		},
		{
			name:    "express in devDependencies",
			content: `{"devDependencies": {"express": "^4.0.0"}}`,
			want:    true,
		},
		{
			name:    "no express",
			content: `{"dependencies": {"react": "^18.0.0"}}`,
			want:    false,
		},
		{
			name:    "empty package.json",
			content: `{}`,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createFile(t, tmpDir, "package.json", tt.content)
			got := s.Detect([]string{file})
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
			os.Remove(file) // Clean up for next test
		})
	}
}

func TestExpressSupplement_Detect_JSFiles(t *testing.T) {
	s := &ExpressSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		want     bool
	}{
		{
			name:     "require with single quotes",
			filename: "app.js",
			content:  `const express = require('express');`,
			want:     true,
		},
		{
			name:     "require with double quotes",
			filename: "app.js",
			content:  `const express = require("express");`,
			want:     true,
		},
		{
			name:     "import with single quotes",
			filename: "app.ts",
			content:  `import express from 'express';`,
			want:     true,
		},
		{
			name:     "import with double quotes",
			filename: "app.ts",
			content:  `import express from "express";`,
			want:     true,
		},
		{
			name:     "no express import",
			filename: "app.js",
			content:  `const http = require('http');`,
			want:     false,
		},
		{
			name:     "python file should not match",
			filename: "main.py",
			content:  `print("express")`,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createFile(t, tmpDir, tt.filename, tt.content)
			got := s.Detect([]string{file})
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
			os.Remove(file)
		})
	}
}

func TestExpressSupplement_Analyze(t *testing.T) {
	s := &ExpressSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	// Create Express app with routes
	routerCode := `
const express = require('express');
const app = express();

app.get('/users', getUsers);
app.post('/users', createUser);
app.get('/users/:id', getUserById);
app.put('/users/:id', updateUser);
app.delete('/users/:id', deleteUser);

// Router example
const router = express.Router();
router.get('/items', listItems);
router.post('/items', createItem);

app.use('/api', router);

app.listen(3000);
`
	routerFile := createFile(t, tmpDir, "app.js", routerCode)

	m := &model.SystemModel{
		Modules: []model.Module{
			{Files: []string{routerFile}},
		},
	}

	err := s.Analyze(m)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if len(m.Endpoints) == 0 {
		t.Fatal("Analyze() should find endpoints")
	}

	// Verify specific endpoints
	expectedEndpoints := map[string]bool{
		"GET:/users":     false,
		"POST:/users":    false,
		"GET:/users/:id": false,
		"PUT:/users/:id": false,
	}

	for _, ep := range m.Endpoints {
		key := ep.Method + ":" + ep.Path
		if _, exists := expectedEndpoints[key]; exists {
			expectedEndpoints[key] = true
		}

		// Verify framework is set
		if ep.Framework != "express" {
			t.Errorf("endpoint %s has framework %s, want express", key, ep.Framework)
		}

		// Verify path params for :id routes
		if ep.Path == "/users/:id" && len(ep.PathParams) == 0 {
			t.Errorf("endpoint %s should have path params", key)
		}
	}

	for key, found := range expectedEndpoints {
		if !found {
			t.Errorf("expected endpoint %s not found", key)
		}
	}
}

// =============================================================================
// FastAPI Supplement Tests
// =============================================================================

func TestFastAPISupplement_Name(t *testing.T) {
	s := &FastAPISupplement{}
	if s.Name() != "fastapi" {
		t.Errorf("Name() = %s, want fastapi", s.Name())
	}
}

func TestFastAPISupplement_Detect(t *testing.T) {
	s := &FastAPISupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		want     bool
	}{
		{
			name:     "requirements.txt with fastapi",
			filename: "requirements.txt",
			content:  "fastapi==0.100.0\nuvicorn==0.23.0",
			want:     true,
		},
		{
			name:     "pyproject.toml with fastapi",
			filename: "pyproject.toml",
			content:  `[project]\ndependencies = ["fastapi>=0.100.0"]`,
			want:     true,
		},
		{
			name:     "python file with fastapi import",
			filename: "main.py",
			content:  `from fastapi import FastAPI\napp = FastAPI()`,
			want:     true,
		},
		{
			name:     "python file with import fastapi",
			filename: "app.py",
			content:  `import fastapi\napp = fastapi.FastAPI()`,
			want:     true,
		},
		{
			name:     "python file without fastapi",
			filename: "utils.py",
			content:  `import os\ndef helper(): pass`,
			want:     false,
		},
		{
			name:     "requirements without fastapi",
			filename: "requirements.txt",
			content:  "django==4.0\ncelery==5.0",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createFile(t, tmpDir, tt.filename, tt.content)
			got := s.Detect([]string{file})
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
			os.Remove(file)
		})
	}
}

func TestFastAPISupplement_Analyze(t *testing.T) {
	s := &FastAPISupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	routerCode := `
from fastapi import FastAPI, APIRouter

app = FastAPI()
router = APIRouter()

@app.get("/health")
def health_check():
    return {"status": "ok"}

@app.get("/users/{user_id}")
async def get_user(user_id: int):
    return {"user_id": user_id}

@router.post("/items")
def create_item(item: dict):
    return item

@router.delete("/items/{item_id}")
async def delete_item(item_id: int):
    return {"deleted": item_id}
`
	routerFile := createFile(t, tmpDir, "main.py", routerCode)

	m := &model.SystemModel{
		Modules: []model.Module{
			{Files: []string{routerFile}},
		},
	}

	err := s.Analyze(m)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if len(m.Endpoints) == 0 {
		t.Fatal("Analyze() should find endpoints")
	}

	// Verify path params extraction
	for _, ep := range m.Endpoints {
		if ep.Path == "/users/{user_id}" && len(ep.PathParams) == 0 {
			t.Error("should extract user_id path param")
		}
		if ep.Framework != "fastapi" {
			t.Errorf("endpoint framework should be fastapi, got %s", ep.Framework)
		}
	}
}

// =============================================================================
// Gin Supplement Tests
// =============================================================================

func TestGinSupplement_Name(t *testing.T) {
	s := &GinSupplement{}
	if s.Name() != "gin" {
		t.Errorf("Name() = %s, want gin", s.Name())
	}
}

func TestGinSupplement_Detect(t *testing.T) {
	s := &GinSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		want     bool
	}{
		{
			name:     "go.mod with gin-gonic",
			filename: "go.mod",
			content:  "module myapp\n\nrequire github.com/gin-gonic/gin v1.9.0",
			want:     true,
		},
		{
			name:     "go file with gin import",
			filename: "main.go",
			content:  "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() {}",
			want:     true,
		},
		{
			name:     "go.mod without gin",
			filename: "go.mod",
			content:  "module myapp\n\nrequire github.com/gorilla/mux v1.8.0",
			want:     false,
		},
		{
			name:     "go file without gin",
			filename: "utils.go",
			content:  "package utils\n\nimport \"fmt\"\n\nfunc Helper() {}",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createFile(t, tmpDir, tt.filename, tt.content)
			got := s.Detect([]string{file})
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
			os.Remove(file)
		})
	}
}

func TestGinSupplement_Analyze(t *testing.T) {
	s := &GinSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	routerCode := `
package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()

	r.GET("/ping", pingHandler)
	r.POST("/users", createUser)
	r.GET("/users/:id", getUser)
	r.PUT("/users/:id", updateUser)
	r.DELETE("/users/:id", deleteUser)

	api := r.Group("/api")
	api.GET("/items", listItems)
	api.POST("/items", createItem)

	r.Run(":8080")
}
`
	routerFile := createFile(t, tmpDir, "main.go", routerCode)

	m := &model.SystemModel{
		Modules: []model.Module{
			{Files: []string{routerFile}},
		},
	}

	err := s.Analyze(m)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if len(m.Endpoints) == 0 {
		t.Fatal("Analyze() should find endpoints")
	}

	// Verify endpoints
	foundMethods := make(map[string]bool)
	for _, ep := range m.Endpoints {
		foundMethods[ep.Method] = true
		if ep.Framework != "gin" {
			t.Errorf("endpoint framework should be gin, got %s", ep.Framework)
		}
		// Check path params
		if ep.Path == "/users/:id" && len(ep.PathParams) == 0 {
			t.Error("should extract id path param")
		}
	}

	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !foundMethods[method] {
			t.Errorf("should find %s method endpoint", method)
		}
	}
}

// =============================================================================
// SpringBoot Supplement Tests
// =============================================================================

func TestSpringBootSupplement_Name(t *testing.T) {
	s := &SpringBootSupplement{}
	if s.Name() != "springboot" {
		t.Errorf("Name() = %s, want springboot", s.Name())
	}
}

func TestSpringBootSupplement_Detect(t *testing.T) {
	s := &SpringBootSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		want     bool
	}{
		{
			name:     "pom.xml with spring-boot",
			filename: "pom.xml",
			content:  "<dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-web</artifactId></dependency>",
			want:     true,
		},
		{
			name:     "build.gradle with spring-boot",
			filename: "build.gradle",
			content:  "dependencies { implementation 'org.springframework.boot:spring-boot-starter-web' }",
			want:     true,
		},
		{
			name:     "build.gradle.kts with spring-boot",
			filename: "build.gradle.kts",
			content:  `implementation("org.springframework.boot:spring-boot-starter-web")`,
			want:     true,
		},
		{
			name:     "java file with @RestController",
			filename: "UserController.java",
			content:  "@RestController\npublic class UserController {}",
			want:     true,
		},
		{
			name:     "java file with @Controller",
			filename: "WebController.java",
			content:  "@Controller\npublic class WebController {}",
			want:     true,
		},
		{
			name:     "java file with @RequestMapping",
			filename: "ApiController.java",
			content:  "@RequestMapping(\"/api\")\npublic class ApiController {}",
			want:     true,
		},
		{
			name:     "java file without spring",
			filename: "Utils.java",
			content:  "public class Utils { public static void helper() {} }",
			want:     false,
		},
		{
			name:     "pom.xml without spring",
			filename: "pom.xml",
			content:  "<dependency><groupId>junit</groupId><artifactId>junit</artifactId></dependency>",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createFile(t, tmpDir, tt.filename, tt.content)
			got := s.Detect([]string{file})
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
			os.Remove(file)
		})
	}
}

func TestSpringBootSupplement_Analyze(t *testing.T) {
	s := &SpringBootSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	controllerCode := `
package com.example.demo;

import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/users")
public class UserController {

    @GetMapping
    public List<User> getUsers() {
        return userService.findAll();
    }

    @GetMapping("/{id}")
    public User getUser(@PathVariable Long id) {
        return userService.findById(id);
    }

    @PostMapping
    public User createUser(@RequestBody User user) {
        return userService.save(user);
    }

    @PutMapping("/{id}")
    public User updateUser(@PathVariable Long id, @RequestBody User user) {
        return userService.update(id, user);
    }

    @DeleteMapping("/{id}")
    public void deleteUser(@PathVariable Long id) {
        userService.delete(id);
    }
}
`
	controllerFile := createFile(t, tmpDir, "UserController.java", controllerCode)

	m := &model.SystemModel{
		Modules: []model.Module{
			{Files: []string{controllerFile}},
		},
	}

	err := s.Analyze(m)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if len(m.Endpoints) == 0 {
		t.Fatal("Analyze() should find endpoints")
	}

	// Verify endpoints have correct framework
	for _, ep := range m.Endpoints {
		if ep.Framework != "springboot" {
			t.Errorf("endpoint framework should be springboot, got %s", ep.Framework)
		}
	}
}

// =============================================================================
// Django Supplement Tests
// =============================================================================

func TestDjangoSupplement_Name(t *testing.T) {
	s := &DjangoSupplement{}
	if s.Name() != "django" {
		t.Errorf("Name() = %s, want django", s.Name())
	}
}

func TestDjangoSupplement_Detect(t *testing.T) {
	s := &DjangoSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		want     bool
	}{
		{
			name:     "requirements.txt with Django",
			filename: "requirements.txt",
			content:  "Django==4.2.0\npsycopg2==2.9.0",
			want:     true,
		},
		{
			name:     "requirements.txt with django lowercase",
			filename: "requirements.txt",
			content:  "django>=4.0\ncelery>=5.0",
			want:     true,
		},
		{
			name:     "requirements.txt with djangorestframework",
			filename: "requirements.txt",
			content:  "djangorestframework==3.14.0",
			want:     true,
		},
		{
			name:     "pyproject.toml with django",
			filename: "pyproject.toml",
			content:  `[project]\ndependencies = ["django>=4.0"]`,
			want:     true,
		},
		{
			name:     "manage.py with django",
			filename: "manage.py",
			content:  "#!/usr/bin/env python\nimport django\nfrom django.core.management import execute_from_command_line",
			want:     true,
		},
		{
			name:     "python file with from django import",
			filename: "views.py",
			content:  "from django.shortcuts import render\ndef index(request): pass",
			want:     true,
		},
		{
			name:     "python file with from rest_framework import",
			filename: "views.py",
			content:  "from rest_framework.views import APIView",
			want:     true,
		},
		{
			name:     "requirements without django",
			filename: "requirements.txt",
			content:  "flask==2.0.0\nrequests==2.28.0",
			want:     false,
		},
		{
			name:     "python file without django",
			filename: "utils.py",
			content:  "import os\ndef helper(): pass",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createFile(t, tmpDir, tt.filename, tt.content)
			got := s.Detect([]string{file})
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
			os.Remove(file)
		})
	}
}

func TestDjangoSupplement_Analyze(t *testing.T) {
	s := &DjangoSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	// Create urls.py
	urlsCode := `
from django.urls import path
from . import views

urlpatterns = [
    path('users/', views.UserListView.as_view()),
    path('users/<int:pk>/', views.UserDetailView.as_view()),
    path('items/', views.list_items, name='item-list'),
]
`
	createFile(t, tmpDir, "urls.py", urlsCode)

	// Create views.py with DRF views
	viewsCode := `
from rest_framework.views import APIView
from rest_framework.decorators import api_view
from rest_framework.response import Response

class UserListView(APIView):
    def get(self, request):
        return Response([])

    def post(self, request):
        return Response({})

class UserDetailView(APIView):
    def get(self, request, pk):
        return Response({})

    def put(self, request, pk):
        return Response({})

    def delete(self, request, pk):
        return Response({})

@api_view(['GET', 'POST'])
def list_items(request):
    if request.method == 'GET':
        return Response([])
    return Response({})
`
	viewsFile := createFile(t, tmpDir, "views.py", viewsCode)

	m := &model.SystemModel{
		Modules: []model.Module{
			{Files: []string{
				filepath.Join(tmpDir, "urls.py"),
				viewsFile,
			}},
		},
	}

	err := s.Analyze(m)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	// Django analysis is complex, just verify no errors and framework is set
	for _, ep := range m.Endpoints {
		if ep.Framework != "django" {
			t.Errorf("endpoint framework should be django, got %s", ep.Framework)
		}
	}
}

func TestDjangoSupplement_ParseMethods(t *testing.T) {
	s := &DjangoSupplement{}

	tests := []struct {
		input string
		want  []string
	}{
		{`'GET', 'POST'`, []string{"GET", "POST"}},
		{`"get", "post", "put"`, []string{"GET", "POST", "PUT"}},
		{`'DELETE'`, []string{"DELETE"}},
		{``, []string{"GET"}}, // Default
	}

	for _, tt := range tests {
		got := s.parseMethods(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseMethods(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i, m := range got {
			if m != tt.want[i] {
				t.Errorf("parseMethods(%q)[%d] = %s, want %s", tt.input, i, m, tt.want[i])
			}
		}
	}
}

// =============================================================================
// NestJS Supplement Tests
// =============================================================================

func TestNestJSSupplement_Name(t *testing.T) {
	s := &NestJSSupplement{}
	if s.Name() != "nestjs" {
		t.Errorf("Name() = %s, want nestjs", s.Name())
	}
}

func TestNestJSSupplement_Detect(t *testing.T) {
	s := &NestJSSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		want     bool
	}{
		{
			name:     "package.json with @nestjs/core",
			filename: "package.json",
			content:  `{"dependencies": {"@nestjs/core": "^10.0.0"}}`,
			want:     true,
		},
		{
			name:     "package.json with @nestjs/common",
			filename: "package.json",
			content:  `{"dependencies": {"@nestjs/common": "^10.0.0"}}`,
			want:     true,
		},
		{
			name:     "ts file with @Controller decorator",
			filename: "users.controller.ts",
			content:  `import { Controller, Get } from '@nestjs/common';\n\n@Controller('users')\nexport class UsersController {}`,
			want:     true,
		},
		{
			name:     "ts file with @nestjs/common import",
			filename: "app.module.ts",
			content:  `import { Module } from '@nestjs/common';`,
			want:     true,
		},
		{
			name:     "package.json without nestjs",
			filename: "package.json",
			content:  `{"dependencies": {"express": "^4.0.0"}}`,
			want:     false,
		},
		{
			name:     "ts file without nestjs",
			filename: "utils.ts",
			content:  `export function helper(): void {}`,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createFile(t, tmpDir, tt.filename, tt.content)
			got := s.Detect([]string{file})
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
			os.Remove(file)
		})
	}
}

func TestNestJSSupplement_Analyze(t *testing.T) {
	s := &NestJSSupplement{}
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	controllerCode := `
import { Controller, Get, Post, Put, Delete, Param, Body } from '@nestjs/common';

@Controller('users')
export class UsersController {
  @Get()
  findAll() {
    return [];
  }

  @Get(':id')
  findOne(@Param('id') id: string) {
    return { id };
  }

  @Post()
  create(@Body() createUserDto: any) {
    return createUserDto;
  }

  @Put(':id')
  update(@Param('id') id: string, @Body() updateUserDto: any) {
    return { id, ...updateUserDto };
  }

  @Delete(':id')
  remove(@Param('id') id: string) {
    return { deleted: id };
  }
}
`
	controllerFile := createFile(t, tmpDir, "users.controller.ts", controllerCode)

	m := &model.SystemModel{
		Modules: []model.Module{
			{Files: []string{controllerFile}},
		},
	}

	err := s.Analyze(m)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if len(m.Endpoints) == 0 {
		t.Fatal("Analyze() should find endpoints")
	}

	// Verify endpoints
	foundMethods := make(map[string]bool)
	for _, ep := range m.Endpoints {
		foundMethods[ep.Method] = true
		if ep.Framework != "nestjs" {
			t.Errorf("endpoint framework should be nestjs, got %s", ep.Framework)
		}
		// Verify base path is included
		if !contains(ep.Path, "users") && ep.Path != "/" {
			t.Errorf("endpoint path should contain 'users', got %s", ep.Path)
		}
	}

	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !foundMethods[method] {
			t.Errorf("should find %s method endpoint", method)
		}
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTempDir(t *testing.T) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "supplements-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return tmpDir
}

func createFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file %s: %v", name, err)
	}
	return path
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestSupplement_Detect_EmptyFiles(t *testing.T) {
	tmpDir := createTempDir(t)
	defer os.RemoveAll(tmpDir)

	// Create empty files
	emptyPkg := createFile(t, tmpDir, "package.json", "")
	emptyReq := createFile(t, tmpDir, "requirements.txt", "")
	emptyMod := createFile(t, tmpDir, "go.mod", "")
	emptyPom := createFile(t, tmpDir, "pom.xml", "")

	files := []string{emptyPkg, emptyReq, emptyMod, emptyPom}

	supplements := []struct {
		name string
		s    interface{ Detect([]string) bool }
	}{
		{"express", &ExpressSupplement{}},
		{"fastapi", &FastAPISupplement{}},
		{"gin", &GinSupplement{}},
		{"springboot", &SpringBootSupplement{}},
		{"django", &DjangoSupplement{}},
		{"nestjs", &NestJSSupplement{}},
	}

	for _, sup := range supplements {
		t.Run(sup.name+"_empty_files", func(t *testing.T) {
			if sup.s.Detect(files) {
				t.Errorf("%s should not detect empty files", sup.name)
			}
		})
	}
}

func TestSupplement_Detect_NonExistentFiles(t *testing.T) {
	nonExistent := []string{
		"/nonexistent/package.json",
		"/nonexistent/requirements.txt",
		"/nonexistent/go.mod",
		"/nonexistent/pom.xml",
	}

	supplements := []struct {
		name string
		s    interface{ Detect([]string) bool }
	}{
		{"express", &ExpressSupplement{}},
		{"fastapi", &FastAPISupplement{}},
		{"gin", &GinSupplement{}},
		{"springboot", &SpringBootSupplement{}},
		{"django", &DjangoSupplement{}},
		{"nestjs", &NestJSSupplement{}},
	}

	for _, sup := range supplements {
		t.Run(sup.name+"_nonexistent_files", func(t *testing.T) {
			if sup.s.Detect(nonExistent) {
				t.Errorf("%s should not detect non-existent files", sup.name)
			}
		})
	}
}

func TestSupplement_Analyze_EmptyModel(t *testing.T) {
	supplements := []struct {
		name string
		s    interface{ Analyze(*model.SystemModel) error }
	}{
		{"express", &ExpressSupplement{}},
		{"fastapi", &FastAPISupplement{}},
		{"gin", &GinSupplement{}},
		{"springboot", &SpringBootSupplement{}},
		{"django", &DjangoSupplement{}},
		{"nestjs", &NestJSSupplement{}},
	}

	for _, sup := range supplements {
		t.Run(sup.name+"_empty_model", func(t *testing.T) {
			m := &model.SystemModel{}
			err := sup.s.Analyze(m)
			if err != nil {
				t.Errorf("%s.Analyze() should not error on empty model: %v", sup.name, err)
			}
		})
	}
}

// Test Express middleware extraction
func TestExtractMiddleware(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{
			line: "app.get('/users', auth, validate, handler)",
			want: []string{"auth", "validate"},
		},
		{
			line: "app.get('/users', handler)",
			want: nil,
		},
		{
			line: "app.get('/users', (req, res) => {})",
			want: nil,
		},
	}

	for _, tt := range tests {
		got := extractMiddleware(tt.line)
		if len(got) != len(tt.want) {
			t.Errorf("extractMiddleware(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"handler", true},
		{"myHandler", true},
		{"_private", true},
		{"controller.method", true},
		{"123invalid", false},
		{"", false},
		{"has space", false},
	}

	for _, tt := range tests {
		got := isValidIdentifier(tt.input)
		if got != tt.want {
			t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
