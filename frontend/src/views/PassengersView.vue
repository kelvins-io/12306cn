<template>
  <div class="page">
    <el-card shadow="never">
      <template #header>
        <div class="hdr">
          <span>常用乘车人</span>
          <el-button type="primary" @click="openAdd">新增</el-button>
        </div>
      </template>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="name" label="姓名" />
        <el-table-column prop="id_type" label="证件类型" />
        <el-table-column prop="id_number" label="证件号" />
        <el-table-column prop="passenger_type" label="类型" width="100" />
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button type="danger" link @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="visible" title="新增乘车人" width="420px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="证件类型">
          <el-select v-model="form.id_type" style="width:100%">
            <el-option label="身份证" value="身份证" />
            <el-option label="护照" value="护照" />
          </el-select>
        </el-form-item>
        <el-form-item label="证件号"><el-input v-model="form.id_number" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.passenger_type" style="width:100%">
            <el-option label="成人" value="成人" />
            <el-option label="儿童" value="儿童" />
            <el-option label="学生" value="学生" />
            <el-option label="军人" value="军人" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '../api'

const list = ref([])
const loading = ref(false)
const visible = ref(false)
const saving = ref(false)
const form = reactive({ name: '', id_type: '身份证', id_number: '', passenger_type: '成人' })

async function load() {
  loading.value = true
  try {
    list.value = await api.listPassengers()
  } finally {
    loading.value = false
  }
}

function openAdd() {
  Object.assign(form, { name: '', id_type: '身份证', id_number: '', passenger_type: '成人' })
  visible.value = true
}

async function save() {
  saving.value = true
  try {
    await api.addPassenger({ ...form })
    ElMessage.success('已添加')
    visible.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`删除乘车人 ${row.name}？`, '提示')
  await api.deletePassenger(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<style scoped>
.hdr {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
