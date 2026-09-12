import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Header } from './components/Header/page'
import { ProtectedRoute } from './components/ProtectedRoute/page'
import { CreateHypothesisPage } from './pages/CreateHypothesis/page'
import { DashboardMainPage } from './pages/DashboardMain/page'
import { EditHypothesisPage } from './pages/EditHypothesis/page'
import { HypothesisPage } from './pages/Hypothesis/page'
import { LoginPage } from './pages/Login/page'
import { ProductPage } from './pages/Product/page'
import { RegisterPage } from './pages/Register/page'

function App() {
  return (
    <BrowserRouter>
      <Header />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <DashboardMainPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/create-hypothesis"
          element={
            <ProtectedRoute>
              <CreateHypothesisPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/edit-hypothesis/:id"
          element={
            <ProtectedRoute>
              <EditHypothesisPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/hypothesis/:id"
          element={
            <ProtectedRoute>
              <HypothesisPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/hypothesis/:hid/product"
          element={
            <ProtectedRoute>
              <ProductPage />
            </ProtectedRoute>
          }
        />
      </Routes>
    </BrowserRouter>
  )
}

export default App
