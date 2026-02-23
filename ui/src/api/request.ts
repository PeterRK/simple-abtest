import axios from 'axios'
import { ElMessage } from 'element-plus'

const service = axios.create({
  baseURL: '/api', // Proxy to http://localhost:8001/api
  timeout: 5000
})

service.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    const msg = error.response?.data?.message || error.message
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

export default service
