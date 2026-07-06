import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { Container } from 'semantic-ui-react';
import { ConfigProvider } from 'antd';
import App from './App';
import Header from './components/Header';
import Footer from './components/Footer';
import 'semantic-ui-css/semantic.min.css';
import 'antd/dist/reset.css';
import './index.css';
import { UserProvider } from './context/User';
import { ToastContainer } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';
import { StatusProvider } from './context/Status';
import { cctAntdTheme } from './ui/theme';
import './i18n';

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <React.StrictMode>
    <ConfigProvider theme={cctAntdTheme}>
      <StatusProvider>
        <UserProvider>
          <BrowserRouter>
            <Header />
            <Container className={'main-content'}>
              <App />
            </Container>
            <ToastContainer position="top-right" toastStyle={{ zIndex: 99999 }} />
            <Footer />
          </BrowserRouter>
        </UserProvider>
      </StatusProvider>
    </ConfigProvider>
  </React.StrictMode>
);
