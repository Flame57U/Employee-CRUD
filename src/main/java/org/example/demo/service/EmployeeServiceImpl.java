package org.example.demo.service;

import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.example.demo.mapper.EmployeeMapper;
import org.example.demo.model.Employee;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.example.demo.service.EmployeeService;
import java.io.Serializable;
import java.util.List;

@Service
public class EmployeeServiceImpl extends ServiceImpl<EmployeeMapper, Employee> implements EmployeeService {

    @Autowired
    @Lazy
    EmployeeMapper employeeMapper;
    // 根据 ID 查询单个员工
    public Employee getEmployeeById(Long id) {
        assertNotNull(employeeMapper.selectById(id),"employee exist");
        return employeeMapper.selectById(id); // 直接使用 selectById 方法
    }

    private void assertNotNull(Employee employee,String s) {
        if (employee == null) {
            throw new IllegalArgumentException("Employee does not exist");
        }
        System.out.println(s);
    }
    // 查询所有员工
    public List<Employee> getAllEmployees() {
        return baseMapper.selectList(null); // 查询所有员工，条件为 null
    }

    public List<Employee> selectAllEmployees() {
        List<Employee> employees = baseMapper.selectList(null);
        if (employees.isEmpty()) { throw new IllegalStateException("没有员工记录"); }
        return employees;
    }
    // 根据条件查询员工
    public List<Employee> getEmployeesByPosition(String position) {
        QueryWrapper<Employee> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("position", position); // 添加查询条件
        return baseMapper.selectList(queryWrapper); // 查询符合条件的员工
    }

    @Override
    public List<Employee> getEmployeesByPage(int PageNo, int PageSize) {
        return List.of();
    }


    public List<Employee> selectdor(){
        QueryWrapper<Employee> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("last_name", "ut"); // 添加查询条件
        return baseMapper.selectList(queryWrapper);
    }
    // 插入员工
    public int insertEmployee(Employee employee) {
        return baseMapper.insert(employee); // 调用 BaseMapper 的 insert 方法
    }

    //主键策略
    public void addateEmployee(Employee employee) {
        if(employee.getId()<0) baseMapper.insert(employee);// 调用 BaseMapper 的 insert 方法
        System.out.println("Employee exists, now update"+ employeeMapper.selectById(employee.getId()));
        baseMapper.updateById(employee);
    }

    // 根据 ID 删除员工
    public int deleteEmployeeById(Serializable id) {
        return baseMapper.deleteById(id); // 调用 deleteById 方法
    }

    // 更新员工信息
    public int updateEmployee(Employee employee) {
        return baseMapper.updateById(employee); // 调用 BaseMapper 的 update 方法
    }

    // 删除所有员工
    public int deleteAllEmployees() {
        return baseMapper.delete(null); // 调用 delete 方法
    }

    // 更新所有员工
    public int updateAllEmployee(Employee updateEntity, String firstname) {
        // 创建 UpdateWrapper，用于指定更新条件
        UpdateWrapper<Employee> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("firstname", firstname); // 例如：根据 firstname 更新
        // 调用 update 方法
        return baseMapper.update(updateEntity, updateWrapper);
    }

    // 升值加薪
    public boolean updateEmployeePosition(Long id, String newPosition) {
        // 根据 ID 查询员工
        Employee employee = this.getById(id); // 使用 IService 的 getById 方法
        if (employee != null) {
            // 设置新的职位
            employee.setPosition(newPosition);
            // 更新员工信息
            return this.updateById(employee); // 使用 ServiceImpl 的 updateById 方法
        }
        return false;
    }

    public IPage<Employee> getEmployeeByPage(int pageNo, int pageSize){
        Page<Employee> page = new Page<>(pageNo, pageSize);
        return employeeMapper.selectPage(page,null);
    }
}