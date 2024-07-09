import torch
import torch.nn as nn


class SimpleNetwork(nn.Module):
    def __init__(self, input_size, hidden_size, output_size):
        super(SimpleNetwork, self).__init__()
        self.fc1 = nn.Linear(input_size, hidden_size)  # first full connection layer
        self.fc2 = nn.Linear(hidden_size, output_size)  # second full connection layer

    def forward(self, x):
        x = torch.relu(self.fc1(x))
        x = self.fc2(x)
        return x


model = SimpleNetwork(784, 128, 10)
x = torch.randn(1, 784)
output = model(x)
