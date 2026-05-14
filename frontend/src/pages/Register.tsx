import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { UserPlus } from 'lucide-react';
import client from '../api/client';
import { useAuth } from '../context/AuthContext';

export const Register: React.FC = () => {
  const [formData, setFormData] = useState({
    name: '',
    email: '',
    password: '',
    bankName: '',
    accountNumber: ''
  });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const { login } = useAuth();

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await client.post('/merchants', formData);
      const res = await client.post('/merchants/login', {
        email: formData.email,
        password: formData.password
      });
      login(res.data.token, res.data.merchant);
      navigate('/dashboard');
    } catch (err: any) {
      setError(err.response?.data || 'Failed to register. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6">
      <div className="w-full max-w-xl bg-surface-container-lowest border border-outline-variant rounded-2xl p-8 shadow-2xl">
        <div className="flex flex-col items-center mb-8">
          <div className="w-12 h-12 rounded-xl bg-primary flex items-center justify-center text-on-primary text-xl font-bold mb-4 shadow-lg">
            L
          </div>
          <h1 className="text-2xl font-bold text-on-surface">Create Account</h1>
          <p className="text-on-surface-variant text-sm mt-2">Join the B2B Payment Platform</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-on-surface-variant mb-1.5">Business Name *</label>
              <input
                type="text"
                name="name"
                value={formData.name}
                onChange={handleChange}
                className="w-full bg-surface-container-low border border-outline-variant rounded-lg px-4 py-2.5 text-on-surface focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-colors"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-on-surface-variant mb-1.5">Email *</label>
              <input
                type="email"
                name="email"
                value={formData.email}
                onChange={handleChange}
                className="w-full bg-surface-container-low border border-outline-variant rounded-lg px-4 py-2.5 text-on-surface focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-colors"
                required
              />
            </div>

            <div className="md:col-span-2">
              <label className="block text-sm font-medium text-on-surface-variant mb-1.5">Password *</label>
              <input
                type="password"
                name="password"
                value={formData.password}
                onChange={handleChange}
                className="w-full bg-surface-container-low border border-outline-variant rounded-lg px-4 py-2.5 text-on-surface focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-colors"
                minLength={6}
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-on-surface-variant mb-1.5">Bank Name *</label>
              <input
                type="text"
                name="bankName"
                value={formData.bankName}
                onChange={handleChange}
                className="w-full bg-surface-container-low border border-outline-variant rounded-lg px-4 py-2.5 text-on-surface focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-colors"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-on-surface-variant mb-1.5">Account Number *</label>
              <input
                type="text"
                name="accountNumber"
                value={formData.accountNumber}
                onChange={handleChange}
                className="w-full bg-surface-container-low border border-outline-variant rounded-lg px-4 py-2.5 text-on-surface focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-colors"
                required
              />
            </div>
          </div>

          {error && (
            <div className="p-3 bg-error-container/30 border border-error/20 rounded-lg mt-4">
              <p className="text-sm text-error">{error}</p>
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-primary hover:bg-primary/90 text-on-primary font-medium py-2.5 rounded-lg transition-colors flex items-center justify-center gap-2 mt-8 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? (
              <div className="w-5 h-5 border-2 border-on-primary/30 border-t-on-primary rounded-full animate-spin" />
            ) : (
              <>
                <UserPlus className="w-4 h-4" />
                Register & Get Sui Address
              </>
            )}
          </button>
        </form>

        <p className="text-center text-sm text-on-surface-variant mt-6">
          Already have an account?{' '}
          <Link to="/login" className="text-primary hover:text-primary/80 font-medium">
            Sign in here
          </Link>
        </p>
      </div>
    </div>
  );
};
