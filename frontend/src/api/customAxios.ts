import axios from "axios";

const apiBaseUrl = import.meta.env.VITE_API_URL || "/api";

export const customInstance = axios.create({
  baseURL: apiBaseUrl,
  timeout: 30000,
});

customInstance.interceptors.request.use((config) => {
  const token = localStorage.getItem("token");
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

customInstance.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem("token");
      if (window.location.pathname !== "/login")
        window.location.href = "/login";
    }
    return Promise.reject(err);
  },
);

export default customInstance;
