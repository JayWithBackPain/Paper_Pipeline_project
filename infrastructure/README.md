# Infrastructure Documentation

AWS 基礎設施配置和部署指南。

## 概述

本專案使用 AWS CloudFormation 管理基礎設施，包含 DynamoDB 資料表、S3 儲存桶、IAM 角色等資源。

## 架構組件

### 核心資源
- **DynamoDB Tables**: Papers 和 Vectors 資料表
- **S3 Buckets**: 原始資料和配置存儲
- **IAM Roles**: Lambda 執行角色和權限
- **CloudWatch**: 日誌群組和告警
- **API Gateway**: REST API 端點

### 網路架構
- **VPC**: 隔離的網路環境 (可選)
- **Security Groups**: 網路存取控制
- **NAT Gateway**: 出站網路存取

## 部署指南

### 快速部署
```bash
# 部署所有基礎設施
make setup-infrastructure

# 部署到特定環境
make setup-infrastructure-env ENV=prod
```

### 手動部署
```bash
# 1. 部署 DynamoDB 資料表
aws cloudformation deploy \
  --template-file cloudformation/dynamodb-tables.yaml \
  --stack-name pipeline-dynamodb-tables \
  --parameter-overrides Environment=dev

# 2. 部署 S3 和 IAM 資源
aws cloudformation deploy \
  --template-file cloudformation/s3-iam.yaml \
  --stack-name pipeline-s3-iam \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides Environment=dev
```

## 資源配置

詳細的 CloudFormation 模板和配置說明請參考各個模板檔案。