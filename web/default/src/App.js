import React, { lazy, Suspense, useContext, useEffect } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import Loading from './components/Loading';
import { PrivateRoute } from './components/PrivateRoute';
import NotFound from './pages/NotFound';
import { API, getLogo, getSystemName, showError, showNotice } from './helpers';
import { UserContext } from './context/User';
import { StatusContext } from './context/Status';

const Home = lazy(() => import('./pages/Home'));
const About = lazy(() => import('./pages/About'));
const Channel = lazy(() => import('./pages/Channel'));
const EditChannel = lazy(() => import('./pages/Channel/EditChannel'));
const Token = lazy(() => import('./pages/Token'));
const EditToken = lazy(() => import('./pages/Token/EditToken'));
const Redemption = lazy(() => import('./pages/Redemption'));
const EditRedemption = lazy(() => import('./pages/Redemption/EditRedemption'));
const User = lazy(() => import('./pages/User'));
const EditUser = lazy(() => import('./pages/User/EditUser'));
const AddUser = lazy(() => import('./pages/User/AddUser'));
const PasswordResetConfirm = lazy(() => import('./components/PasswordResetConfirm'));
const LoginForm = lazy(() => import('./components/LoginForm'));
const RegisterForm = lazy(() => import('./components/RegisterForm'));
const PasswordResetForm = lazy(() => import('./components/PasswordResetForm'));
const GitHubOAuth = lazy(() => import('./components/GitHubOAuth'));
const LarkOAuth = lazy(() => import('./components/LarkOAuth'));
const Setting = lazy(() => import('./pages/Setting'));
const TopUp = lazy(() => import('./pages/TopUp'));
const Log = lazy(() => import('./pages/Log'));
const Chat = lazy(() => import('./pages/Chat'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Fallback = lazy(() => import('./pages/Fallback'));

function App() {
  const [, userDispatch] = useContext(UserContext);
  const [, statusDispatch] = useContext(StatusContext);

  const loadUser = () => {
    let user = localStorage.getItem('user');
    if (user) {
      let data = JSON.parse(user);
      userDispatch({ type: 'login', payload: data });
    }
  };
  const loadStatus = async () => {
    try {
      const res = await API.get('/api/status');
      const { success, message, data } = res.data || {}; // Add default empty object
      if (success && data) {
        // Check data exists
        localStorage.setItem('status', JSON.stringify(data));
        statusDispatch({ type: 'set', payload: data });
        localStorage.setItem('system_name', data.system_name);
        localStorage.setItem('logo', data.logo);
        localStorage.setItem('footer_html', data.footer_html);
        localStorage.setItem('quota_per_unit', data.quota_per_unit);
        localStorage.setItem('display_in_currency', data.display_in_currency);
        if (data.chat_link) {
          localStorage.setItem('chat_link', data.chat_link);
        } else {
          localStorage.removeItem('chat_link');
        }
        if (
          data.version !== import.meta.env.VITE_APP_VERSION &&
          data.version !== 'v0.0.0' &&
          import.meta.env.VITE_APP_VERSION !== ''
        ) {
          showNotice(
            `新版本可用：${data.version}，请使用快捷键 Shift + F5 刷新页面`
          );
        }
      } else {
        showError(message || '无法正常连接至服务器！');
      }
    } catch (error) {
      showError(error.message || '无法正常连接至服务器！');
    }
  };

  useEffect(() => {
    loadUser();
    loadStatus().then();
    let systemName = getSystemName();
    if (systemName) {
      document.title = systemName;
    }
    let logo = getLogo();
    if (logo) {
      let linkElement = document.querySelector("link[rel~='icon']");
      if (linkElement) {
        linkElement.href = logo;
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <Routes>
      <Route
        path='/'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <Home />
          </Suspense>
        }
      />
      <Route
        path='/channel'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <Channel />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/channel/edit/:id'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditChannel />
          </Suspense>
        }
      />
      <Route
        path='/channel/add'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditChannel />
          </Suspense>
        }
      />
      <Route
        path='/token'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <Token />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/token/edit/:id'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditToken />
          </Suspense>
        }
      />
      <Route
        path='/token/add'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditToken />
          </Suspense>
        }
      />
      <Route
        path='/redemption'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <Redemption />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/redemption/edit/:id'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditRedemption />
          </Suspense>
        }
      />
      <Route
        path='/redemption/add'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditRedemption />
          </Suspense>
        }
      />
      <Route
        path='/user'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <User />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/user/edit/:id'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditUser />
          </Suspense>
        }
      />
      <Route
        path='/user/edit'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <EditUser />
          </Suspense>
        }
      />
      <Route
        path='/user/add'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <AddUser />
          </Suspense>
        }
      />
      <Route
        path='/user/reset'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <PasswordResetConfirm />
          </Suspense>
        }
      />
      <Route
        path='/login'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <LoginForm />
          </Suspense>
        }
      />
      <Route
        path='/register'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <RegisterForm />
          </Suspense>
        }
      />
      <Route
        path='/reset'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <PasswordResetForm />
          </Suspense>
        }
      />
      <Route
        path='/oauth/github'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <GitHubOAuth />
          </Suspense>
        }
      />
      <Route
        path='/oauth/lark'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <LarkOAuth />
          </Suspense>
        }
      />
      <Route
        path='/setting'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <Setting />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/topup'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <TopUp />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/log'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <Log />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/about'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <About />
          </Suspense>
        }
      />
      <Route
        path='/chat'
        element={
          <Suspense fallback={<Loading></Loading>}>
            <Chat />
          </Suspense>
        }
      />
      <Route
        path='/dashboard'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <Dashboard />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route
        path='/fallback'
        element={
          <PrivateRoute>
            <Navigate to='/fallback/status' replace />
          </PrivateRoute>
        }
      />
      <Route
        path='/fallback/:panel'
        element={
          <PrivateRoute>
            <Suspense fallback={<Loading></Loading>}>
              <Fallback />
            </Suspense>
          </PrivateRoute>
        }
      />
      <Route path='*' element={<NotFound />} />
    </Routes>
  );
}

export default App;
